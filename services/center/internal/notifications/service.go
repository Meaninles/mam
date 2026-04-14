package notifications

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	apperrors "mare/services/center/internal/errors"
	notificationdto "mare/shared/contracts/dto/notification"
)

const (
	KindActionRequired = "ACTION_REQUIRED"
	KindReminder       = "REMINDER"

	SourceTypeIssue = "ISSUE"
	SourceTypeJob   = "JOB"

	LifecycleActive = "ACTIVE"
	LifecycleStale  = "STALE"

	SeverityCritical = "CRITICAL"
	SeverityWarning  = "WARNING"
	SeverityInfo     = "INFO"
	SeveritySuccess  = "SUCCESS"

	TargetIssues     = "issues"
	TargetTaskCenter = "task-center"
	TargetFileCenter = "file-center"
	TargetStorage    = "storage-nodes"
	TargetImport     = "import-center"

	StreamEventCreated = "NOTIFICATION_CREATED"
	StreamEventUpdated = "NOTIFICATION_UPDATED"
	StreamEventStale   = "NOTIFICATION_STALE"
)

type Record = notificationdto.Record
type ListResponse = notificationdto.ListResponse
type StreamEvent = notificationdto.StreamEvent

type ListQuery struct {
	Page         int
	PageSize     int
	Kind         string
	SearchText   string
	IncludeStale bool
}

type Service struct {
	pool   *pgxpool.Pool
	now    func() time.Time
	broker *Broker
}

func NewService(pool *pgxpool.Pool) *Service {
	return &Service{
		pool:   pool,
		now:    time.Now,
		broker: NewBroker(),
	}
}

func (s *Service) Subscribe() (<-chan notificationdto.StreamEvent, func()) {
	return s.broker.Subscribe()
}

func (s *Service) ListNotifications(ctx context.Context, query ListQuery) (notificationdto.ListResponse, error) {
	page := query.Page
	if page <= 0 {
		page = 1
	}
	pageSize := query.PageSize
	if pageSize <= 0 {
		pageSize = 50
	}

	whereParts := []string{"TRUE"}
	args := make([]any, 0)
	addArg := func(value any) string {
		args = append(args, value)
		return fmt.Sprintf("$%d", len(args))
	}

	if !query.IncludeStale {
		whereParts = append(whereParts, fmt.Sprintf("lifecycle_status = %s", addArg(LifecycleActive)))
	}
	if kind := strings.TrimSpace(query.Kind); kind != "" {
		whereParts = append(whereParts, fmt.Sprintf("kind = %s", addArg(kind)))
	}
	if text := strings.TrimSpace(query.SearchText); text != "" {
		placeholder := addArg("%" + text + "%")
		whereParts = append(whereParts, fmt.Sprintf(`(
			title ILIKE %s OR
			summary ILIKE %s OR
			object_label ILIKE %s OR
			COALESCE(source_payload ->> 'sourceLabel', '') ILIKE %s OR
			COALESCE(source_payload ->> 'routeLabel', '') ILIKE %s
		)`, placeholder, placeholder, placeholder, placeholder, placeholder))
	}

	whereSQL := strings.Join(whereParts, " AND ")
	var total int
	if err := s.pool.QueryRow(ctx, "SELECT COUNT(*) FROM notifications WHERE "+whereSQL, args...).Scan(&total); err != nil {
		return notificationdto.ListResponse{}, err
	}

	args = append(args, pageSize, (page-1)*pageSize)
	rows, err := s.pool.Query(ctx, `
		SELECT
			id, source_type, source_id, job_id, issue_id, library_id, kind, lifecycle_status,
			default_target_kind, title, summary, severity, object_label,
			source_payload, capabilities_payload, jump_params, created_at, updated_at
		FROM notifications
		WHERE `+whereSQL+`
		ORDER BY updated_at DESC, created_at DESC
		LIMIT $`+fmt.Sprint(len(args)-1)+` OFFSET $`+fmt.Sprint(len(args))+`
	`, args...)
	if err != nil {
		return notificationdto.ListResponse{}, err
	}
	defer rows.Close()

	items := make([]notificationdto.Record, 0)
	for rows.Next() {
		record, err := scanRecord(rows)
		if err != nil {
			return notificationdto.ListResponse{}, err
		}
		items = append(items, record)
	}
	if err := rows.Err(); err != nil {
		return notificationdto.ListResponse{}, err
	}

	return notificationdto.ListResponse{
		Items:    items,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	}, nil
}

func (s *Service) SyncJobNotifications(ctx context.Context, jobID string) error {
	if strings.TrimSpace(jobID) == "" {
		return nil
	}

	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	job, err := s.loadJobProjection(ctx, tx, jobID)
	if err != nil {
		return err
	}
	activeIssues, err := s.loadIssueProjections(ctx, tx, jobID)
	if err != nil {
		return err
	}

	changes := make([]notificationdto.StreamEvent, 0)
	activeIssueIDs := make(map[string]struct{}, len(activeIssues))
	for _, issue := range activeIssues {
		activeIssueIDs[issue.ID] = struct{}{}
		event, changed, err := s.upsertIssueNotice(ctx, tx, job, issue)
		if err != nil {
			return err
		}
		if changed {
			changes = append(changes, event)
		}
	}

	staleEvents, err := s.staleIssueNotices(ctx, tx, jobID, activeIssueIDs)
	if err != nil {
		return err
	}
	changes = append(changes, staleEvents...)

	jobEvent, changed, err := s.syncJobReminderNotice(ctx, tx, job)
	if err != nil {
		return err
	}
	if changed {
		changes = append(changes, jobEvent)
	}

	if err := tx.Commit(ctx); err != nil {
		return err
	}
	for _, event := range changes {
		s.broker.Publish(event)
	}
	return nil
}

func (s *Service) SyncIssueNotification(ctx context.Context, issueID string) error {
	if strings.TrimSpace(issueID) == "" {
		return nil
	}

	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	issue, found, err := s.loadIssueProjectionByID(ctx, tx, issueID)
	if err != nil {
		return err
	}

	changes := make([]notificationdto.StreamEvent, 0, 1)
	if found {
		event, changed, err := s.upsertIssueNotice(ctx, tx, jobProjection{}, issue)
		if err != nil {
			return err
		}
		if changed {
			changes = append(changes, event)
		}
	} else {
		now := s.now().UTC()
		tag, err := tx.Exec(ctx, `
			UPDATE notifications
			SET lifecycle_status = 'STALE',
			    stale_at = $2,
			    updated_at = $2
			WHERE source_type = 'ISSUE'
			  AND source_id = $1
			  AND lifecycle_status <> 'STALE'
		`, issueID, now)
		if err != nil {
			return err
		}
		if tag.RowsAffected() > 0 {
			status := LifecycleStale
			changes = append(changes, notificationdto.StreamEvent{
				EventID:         buildCode("notification-stream", now),
				Topic:           "NOTIFICATION",
				EventType:       StreamEventStale,
				NotificationID:  noticeID(SourceTypeIssue, issueID),
				LifecycleStatus: &status,
				CreatedAt:       now.Format(time.RFC3339),
			})
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return err
	}
	for _, event := range changes {
		s.broker.Publish(event)
	}
	return nil
}

type jobProjection struct {
	ID                 string
	LibraryID          *string
	Status             string
	Title              string
	Summary            string
	SourceDomain       string
	SourceSnapshot     []byte
	IssueCount         int
	LatestErrorMessage *string
	OutcomeSummary     *string
	UpdatedAt          time.Time
}

type issueProjection struct {
	ID             string
	LibraryID      *string
	IssueCategory  string
	Nature         string
	SourceDomain   string
	Severity       string
	Title          string
	Summary        string
	ObjectLabel    string
	SourceSnapshot []byte
	UpdatedAt      time.Time
}

func (s *Service) loadJobProjection(ctx context.Context, tx pgx.Tx, jobID string) (jobProjection, error) {
	row := tx.QueryRow(ctx, `
		SELECT id, library_id, status, title, summary, source_domain, source_snapshot, issue_count, latest_error_message, outcome_summary, updated_at
		FROM jobs
		WHERE id = $1
	`, jobID)

	var result jobProjection
	if err := row.Scan(
		&result.ID,
		&result.LibraryID,
		&result.Status,
		&result.Title,
		&result.Summary,
		&result.SourceDomain,
		&result.SourceSnapshot,
		&result.IssueCount,
		&result.LatestErrorMessage,
		&result.OutcomeSummary,
		&result.UpdatedAt,
	); err != nil {
		if err == pgx.ErrNoRows {
			return jobProjection{}, apperrors.NotFound("作业不存在")
		}
		return jobProjection{}, err
	}
	return result, nil
}

func (s *Service) loadIssueProjections(ctx context.Context, tx pgx.Tx, jobID string) ([]issueProjection, error) {
	rows, err := tx.Query(ctx, `
		SELECT id, library_id, issue_category, nature, source_domain, severity, title, summary, object_label, source_snapshot, updated_at
		FROM issues
		WHERE COALESCE(source_snapshot ->> 'taskId', '') = $1
		  AND status IN ('OPEN', 'AWAITING_CONFIRMATION', 'IN_PROGRESS')
		ORDER BY updated_at DESC
	`, jobID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]issueProjection, 0)
	for rows.Next() {
		var item issueProjection
		if err := rows.Scan(
			&item.ID,
			&item.LibraryID,
			&item.IssueCategory,
			&item.Nature,
			&item.SourceDomain,
			&item.Severity,
			&item.Title,
			&item.Summary,
			&item.ObjectLabel,
			&item.SourceSnapshot,
			&item.UpdatedAt,
		); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Service) loadIssueProjectionByID(ctx context.Context, tx pgx.Tx, issueID string) (issueProjection, bool, error) {
	row := tx.QueryRow(ctx, `
		SELECT id, library_id, issue_category, nature, source_domain, severity, title, summary, object_label, source_snapshot, updated_at
		FROM issues
		WHERE id = $1
		  AND status IN ('OPEN', 'AWAITING_CONFIRMATION', 'IN_PROGRESS')
	`, issueID)

	var item issueProjection
	if err := row.Scan(
		&item.ID,
		&item.LibraryID,
		&item.IssueCategory,
		&item.Nature,
		&item.SourceDomain,
		&item.Severity,
		&item.Title,
		&item.Summary,
		&item.ObjectLabel,
		&item.SourceSnapshot,
		&item.UpdatedAt,
	); err != nil {
		if err == pgx.ErrNoRows {
			return issueProjection{}, false, nil
		}
		return issueProjection{}, false, err
	}
	return item, true, nil
}

func (s *Service) upsertIssueNotice(ctx context.Context, tx pgx.Tx, job jobProjection, issue issueProjection) (notificationdto.StreamEvent, bool, error) {
	source, jumpParams, capabilities := buildIssuePayload(issue)
	var jobID *string
	if strings.TrimSpace(job.ID) != "" {
		jobID = &job.ID
	}
	return s.upsertNotice(ctx, tx, upsertInput{
		SourceType:        SourceTypeIssue,
		SourceID:          issue.ID,
		JobID:             jobID,
		IssueID:           &issue.ID,
		LibraryID:         issue.LibraryID,
		Kind:              KindActionRequired,
		LifecycleStatus:   LifecycleActive,
		DefaultTargetKind: TargetIssues,
		Title:             issue.Title,
		Summary:           issue.Summary,
		Severity:          issue.Severity,
		ObjectLabel:       issue.ObjectLabel,
		Source:            source,
		Capabilities:      capabilities,
		JumpParams:        jumpParams,
		UpdatedAt:         issue.UpdatedAt.UTC(),
	})
}

func (s *Service) staleIssueNotices(ctx context.Context, tx pgx.Tx, jobID string, active map[string]struct{}) ([]notificationdto.StreamEvent, error) {
	rows, err := tx.Query(ctx, `
		SELECT id, issue_id
		FROM notifications
		WHERE source_type = 'ISSUE'
		  AND job_id = $1
		  AND lifecycle_status <> 'STALE'
	`, jobID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	now := s.now().UTC()
	targetIDs := make([]string, 0)
	for rows.Next() {
		var notificationID string
		var issueID *string
		if err := rows.Scan(&notificationID, &issueID); err != nil {
			return nil, err
		}
		if issueID != nil {
			if _, ok := active[*issueID]; ok {
				continue
			}
		}
		targetIDs = append(targetIDs, notificationID)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	events := make([]notificationdto.StreamEvent, 0, len(targetIDs))
	for _, notificationID := range targetIDs {
		if _, err := tx.Exec(ctx, `
			UPDATE notifications
			SET lifecycle_status = 'STALE',
			    stale_at = $2,
			    updated_at = $2
			WHERE id = $1
		`, notificationID, now); err != nil {
			return nil, err
		}
		status := LifecycleStale
		events = append(events, notificationdto.StreamEvent{
			EventID:         buildCode("notification-stream", now),
			Topic:           "NOTIFICATION",
			EventType:       StreamEventStale,
			NotificationID:  notificationID,
			LifecycleStatus: &status,
			CreatedAt:       now.Format(time.RFC3339),
		})
	}
	return events, nil
}

func (s *Service) syncJobReminderNotice(ctx context.Context, tx pgx.Tx, job jobProjection) (notificationdto.StreamEvent, bool, error) {
	if shouldExposeJobReminder(job.Status, job.IssueCount) {
		source, jumpParams, capabilities := buildJobPayload(job)
		return s.upsertNotice(ctx, tx, upsertInput{
			SourceType:        SourceTypeJob,
			SourceID:          job.ID,
			JobID:             &job.ID,
			LibraryID:         job.LibraryID,
			Kind:              KindReminder,
			LifecycleStatus:   LifecycleActive,
			DefaultTargetKind: TargetTaskCenter,
			Title:             deriveJobReminderTitle(job),
			Summary:           deriveJobReminderSummary(job),
			Severity:          deriveJobReminderSeverity(job.Status),
			ObjectLabel:       deriveJobObjectLabel(job),
			Source:            source,
			Capabilities:      capabilities,
			JumpParams:        jumpParams,
			UpdatedAt:         job.UpdatedAt.UTC(),
		})
	}

	now := s.now().UTC()
	tag, err := tx.Exec(ctx, `
		UPDATE notifications
		SET lifecycle_status = 'STALE',
		    stale_at = $2,
		    updated_at = $2
		WHERE source_type = 'JOB'
		  AND source_id = $1
		  AND lifecycle_status <> 'STALE'
	`, job.ID, now)
	if err != nil {
		return notificationdto.StreamEvent{}, false, err
	}
	if tag.RowsAffected() == 0 {
		return notificationdto.StreamEvent{}, false, nil
	}
	status := LifecycleStale
	return notificationdto.StreamEvent{
		EventID:         buildCode("notification-stream", now),
		Topic:           "NOTIFICATION",
		EventType:       StreamEventStale,
		NotificationID:  noticeID(SourceTypeJob, job.ID),
		LifecycleStatus: &status,
		CreatedAt:       now.Format(time.RFC3339),
	}, true, nil
}

type upsertInput struct {
	SourceType        string
	SourceID          string
	JobID             *string
	IssueID           *string
	LibraryID         *string
	Kind              string
	LifecycleStatus   string
	DefaultTargetKind string
	Title             string
	Summary           string
	Severity          string
	ObjectLabel       string
	Source            notificationdto.Source
	Capabilities      notificationdto.Capabilities
	JumpParams        notificationdto.JumpParams
	UpdatedAt         time.Time
}

func (s *Service) upsertNotice(ctx context.Context, tx pgx.Tx, input upsertInput) (notificationdto.StreamEvent, bool, error) {
	id := noticeID(input.SourceType, input.SourceID)
	sourceJSON, err := json.Marshal(input.Source)
	if err != nil {
		return notificationdto.StreamEvent{}, false, err
	}
	capabilitiesJSON, err := json.Marshal(input.Capabilities)
	if err != nil {
		return notificationdto.StreamEvent{}, false, err
	}
	jumpJSON, err := json.Marshal(input.JumpParams)
	if err != nil {
		return notificationdto.StreamEvent{}, false, err
	}

	existing, found, err := s.loadNoticeRow(ctx, tx, id)
	if err != nil {
		return notificationdto.StreamEvent{}, false, err
	}
	if !found {
		_, err := tx.Exec(ctx, `
			INSERT INTO notifications (
				id, source_type, source_id, job_id, issue_id, library_id, kind, lifecycle_status,
				default_target_kind, title, summary, severity, object_label,
				source_payload, capabilities_payload, jump_params, created_at, updated_at
			) VALUES (
				$1, $2, $3, $4, $5, $6, $7, $8,
				$9, $10, $11, $12, $13,
				$14, $15, $16, $17, $17
			)
		`, id, input.SourceType, input.SourceID, input.JobID, input.IssueID, input.LibraryID, input.Kind, input.LifecycleStatus,
			input.DefaultTargetKind, input.Title, input.Summary, input.Severity, input.ObjectLabel,
			sourceJSON, capabilitiesJSON, jumpJSON, input.UpdatedAt)
		if err != nil {
			return notificationdto.StreamEvent{}, false, err
		}
		status := input.LifecycleStatus
		return notificationdto.StreamEvent{
			EventID:         buildCode("notification-stream", input.UpdatedAt),
			Topic:           "NOTIFICATION",
			EventType:       StreamEventCreated,
			NotificationID:  id,
			LifecycleStatus: &status,
			CreatedAt:       input.UpdatedAt.Format(time.RFC3339),
		}, true, nil
	}

	if existing.matches(input, sourceJSON, capabilitiesJSON, jumpJSON) {
		return notificationdto.StreamEvent{}, false, nil
	}

	_, err = tx.Exec(ctx, `
		UPDATE notifications
		SET job_id = $2,
		    issue_id = $3,
		    library_id = $4,
		    kind = $5,
		    lifecycle_status = $6,
		    default_target_kind = $7,
		    title = $8,
		    summary = $9,
		    severity = $10,
		    object_label = $11,
		    source_payload = $12,
		    capabilities_payload = $13,
		    jump_params = $14,
		    stale_at = CASE WHEN $6 = 'STALE' THEN COALESCE(stale_at, $15) ELSE NULL END,
		    updated_at = $15
		WHERE id = $1
	`, id, input.JobID, input.IssueID, input.LibraryID, input.Kind, input.LifecycleStatus, input.DefaultTargetKind,
		input.Title, input.Summary, input.Severity, input.ObjectLabel, sourceJSON, capabilitiesJSON, jumpJSON, input.UpdatedAt)
	if err != nil {
		return notificationdto.StreamEvent{}, false, err
	}

	status := input.LifecycleStatus
	eventType := StreamEventUpdated
	if input.LifecycleStatus == LifecycleStale {
		eventType = StreamEventStale
	}
	return notificationdto.StreamEvent{
		EventID:         buildCode("notification-stream", input.UpdatedAt),
		Topic:           "NOTIFICATION",
		EventType:       eventType,
		NotificationID:  id,
		LifecycleStatus: &status,
		CreatedAt:       input.UpdatedAt.Format(time.RFC3339),
	}, true, nil
}

type noticeRow struct {
	JobID             *string
	IssueID           *string
	LibraryID         *string
	Kind              string
	LifecycleStatus   string
	DefaultTargetKind string
	Title             string
	Summary           string
	Severity          string
	ObjectLabel       string
	SourcePayload     []byte
	CapabilitiesJSON  []byte
	JumpParamsJSON    []byte
}

func (s *Service) loadNoticeRow(ctx context.Context, tx pgx.Tx, id string) (noticeRow, bool, error) {
	row := tx.QueryRow(ctx, `
		SELECT job_id, issue_id, library_id, kind, lifecycle_status, default_target_kind, title, summary, severity, object_label, source_payload, capabilities_payload, jump_params
		FROM notifications
		WHERE id = $1
	`, id)

	var item noticeRow
	if err := row.Scan(
		&item.JobID,
		&item.IssueID,
		&item.LibraryID,
		&item.Kind,
		&item.LifecycleStatus,
		&item.DefaultTargetKind,
		&item.Title,
		&item.Summary,
		&item.Severity,
		&item.ObjectLabel,
		&item.SourcePayload,
		&item.CapabilitiesJSON,
		&item.JumpParamsJSON,
	); err != nil {
		if err == pgx.ErrNoRows {
			return noticeRow{}, false, nil
		}
		return noticeRow{}, false, err
	}
	return item, true, nil
}

func (r noticeRow) matches(input upsertInput, sourceJSON []byte, capabilitiesJSON []byte, jumpJSON []byte) bool {
	return optionalEqual(r.JobID, input.JobID) &&
		optionalEqual(r.IssueID, input.IssueID) &&
		optionalEqual(r.LibraryID, input.LibraryID) &&
		r.Kind == input.Kind &&
		r.LifecycleStatus == input.LifecycleStatus &&
		r.DefaultTargetKind == input.DefaultTargetKind &&
		r.Title == input.Title &&
		r.Summary == input.Summary &&
		r.Severity == input.Severity &&
		r.ObjectLabel == input.ObjectLabel &&
		string(r.SourcePayload) == string(sourceJSON) &&
		string(r.CapabilitiesJSON) == string(capabilitiesJSON) &&
		string(r.JumpParamsJSON) == string(jumpJSON)
}

func scanRecord(row pgx.Row) (notificationdto.Record, error) {
	var item notificationdto.Record
	var sourceJSON []byte
	var capabilitiesJSON []byte
	var jumpJSON []byte
	var createdAt time.Time
	var updatedAt time.Time
	if err := row.Scan(
		&item.ID,
		&item.SourceType,
		&item.SourceID,
		&item.JobID,
		&item.IssueID,
		&item.LibraryID,
		&item.Kind,
		&item.LifecycleStatus,
		&item.DefaultTargetKind,
		&item.Title,
		&item.Summary,
		&item.Severity,
		&item.ObjectLabel,
		&sourceJSON,
		&capabilitiesJSON,
		&jumpJSON,
		&createdAt,
		&updatedAt,
	); err != nil {
		return notificationdto.Record{}, err
	}
	if len(sourceJSON) > 0 {
		_ = json.Unmarshal(sourceJSON, &item.Source)
	}
	if len(capabilitiesJSON) > 0 {
		_ = json.Unmarshal(capabilitiesJSON, &item.Capabilities)
	}
	if len(jumpJSON) > 0 {
		_ = json.Unmarshal(jumpJSON, &item.JumpParams)
	}
	item.CreatedAt = createdAt.UTC().Format(time.RFC3339)
	item.UpdatedAt = updatedAt.UTC().Format(time.RFC3339)
	return item, nil
}

func buildIssuePayload(issue issueProjection) (notificationdto.Source, notificationdto.JumpParams, notificationdto.Capabilities) {
	var snapshot struct {
		TaskID      *string `json:"taskId"`
		TaskItemID  *string `json:"taskItemId"`
		EntryID     *string `json:"entryId"`
		EndpointID  *string `json:"endpointId"`
		Path        *string `json:"path"`
		SourceLabel *string `json:"sourceLabel"`
		RouteLabel  *string `json:"routeLabel"`
	}
	_ = json.Unmarshal(issue.SourceSnapshot, &snapshot)

	label := deriveIssueLabel(issue.Title, snapshot.SourceLabel)
	sourceDomain := issue.SourceDomain
	return notificationdto.Source{
			SourceDomain:      ptr("ISSUE_CENTER"),
			IssueCategory:     &issue.IssueCategory,
			IssueNature:       &issue.Nature,
			IssueSourceDomain: &sourceDomain,
			TaskID:            snapshot.TaskID,
			TaskItemID:        snapshot.TaskItemID,
			FileNodeID:        snapshot.EntryID,
			EndpointID:        snapshot.EndpointID,
			Path:              snapshot.Path,
			SourceLabel:       snapshot.SourceLabel,
			RouteLabel:        snapshot.RouteLabel,
		},
		notificationdto.JumpParams{
			Kind:         TargetIssues,
			IssueID:      &issue.ID,
			TaskID:       snapshot.TaskID,
			TaskItemID:   snapshot.TaskItemID,
			LibraryID:    issue.LibraryID,
			EndpointID:   snapshot.EndpointID,
			FileNodeID:   snapshot.EntryID,
			Path:         snapshot.Path,
			SourceDomain: &sourceDomain,
			Label:        &label,
		},
		notificationdto.Capabilities{
			CanMarkRead:         true,
			CanOpenIssueCenter:  true,
			CanOpenTaskCenter:   snapshot.TaskID != nil,
			CanOpenFileCenter:   issue.LibraryID != nil,
			CanOpenStorageNodes: snapshot.EndpointID != nil || snapshot.Path != nil,
			CanOpenImportCenter: false,
		}
}

func buildJobPayload(job jobProjection) (notificationdto.Source, notificationdto.JumpParams, notificationdto.Capabilities) {
	var snapshot struct {
		DirectoryID *string `json:"directoryId"`
		EntryID     *string `json:"entryId"`
		EndpointID  *string `json:"endpointId"`
		Path        *string `json:"path"`
	}
	_ = json.Unmarshal(job.SourceSnapshot, &snapshot)

	fileNodeID := firstNonNil(snapshot.EntryID, snapshot.DirectoryID)
	label := "查看任务结果"
	return notificationdto.Source{
			SourceDomain: ptr("TASK_CENTER"),
			TaskID:       &job.ID,
			FileNodeID:   fileNodeID,
			EndpointID:   snapshot.EndpointID,
			Path:         snapshot.Path,
			SourceLabel:  ptr("任务中心"),
			RouteLabel:   ptr("任务中心 / 任务详情"),
		},
		notificationdto.JumpParams{
			Kind:       TargetTaskCenter,
			TaskID:     &job.ID,
			LibraryID:  job.LibraryID,
			EndpointID: snapshot.EndpointID,
			FileNodeID: fileNodeID,
			Path:       snapshot.Path,
			Label:      &label,
		},
		notificationdto.Capabilities{
			CanMarkRead:         true,
			CanOpenIssueCenter:  job.IssueCount > 0,
			CanOpenTaskCenter:   true,
			CanOpenFileCenter:   job.LibraryID != nil,
			CanOpenStorageNodes: snapshot.EndpointID != nil || snapshot.Path != nil,
			CanOpenImportCenter: job.SourceDomain == "IMPORT_CENTER",
		}
}

func shouldExposeJobReminder(status string, issueCount int) bool {
	switch status {
	case "COMPLETED":
		return true
	case "FAILED", "PARTIAL_SUCCESS", "CANCELED":
		return issueCount == 0
	default:
		return false
	}
}

func deriveJobReminderTitle(job jobProjection) string {
	switch job.Status {
	case "COMPLETED":
		return job.Title + " 已完成"
	case "PARTIAL_SUCCESS":
		return job.Title + " 部分成功"
	case "FAILED":
		return job.Title + " 执行失败"
	case "CANCELED":
		return job.Title + " 已取消"
	default:
		return job.Title
	}
}

func deriveJobReminderSummary(job jobProjection) string {
	if value := strings.TrimSpace(optionalValue(job.OutcomeSummary)); value != "" {
		return value
	}
	if value := strings.TrimSpace(job.Summary); value != "" {
		switch job.Status {
		case "COMPLETED":
			return value + "，作业已完成"
		case "PARTIAL_SUCCESS":
			return value + "，作业部分成功"
		case "FAILED":
			return value + "，作业执行失败"
		case "CANCELED":
			return value + "，作业已取消"
		}
		return value
	}
	if value := strings.TrimSpace(optionalValue(job.LatestErrorMessage)); value != "" {
		return value
	}
	return "任务状态已更新"
}

func deriveJobReminderSeverity(status string) string {
	switch status {
	case "COMPLETED":
		return SeveritySuccess
	case "FAILED":
		return SeverityCritical
	default:
		return SeverityWarning
	}
}

func deriveJobObjectLabel(job jobProjection) string {
	var snapshot struct {
		LibraryName string  `json:"libraryName"`
		Path        *string `json:"path"`
	}
	_ = json.Unmarshal(job.SourceSnapshot, &snapshot)
	if strings.TrimSpace(snapshot.LibraryName) != "" && snapshot.Path != nil && strings.TrimSpace(*snapshot.Path) != "" {
		return snapshot.LibraryName + " / " + strings.TrimSpace(*snapshot.Path)
	}
	if snapshot.Path != nil && strings.TrimSpace(*snapshot.Path) != "" {
		return strings.TrimSpace(*snapshot.Path)
	}
	return job.Title
}

func deriveIssueLabel(title string, sourceLabel *string) string {
	if sourceLabel != nil && strings.TrimSpace(*sourceLabel) != "" {
		return "按来源查看异常：" + strings.TrimSpace(*sourceLabel)
	}
	return "定位异常：" + strings.TrimSpace(title)
}

func optionalEqual(left *string, right *string) bool {
	return optionalValue(left) == optionalValue(right)
}

func optionalValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func ptr(value string) *string {
	return &value
}

func firstNonNil(values ...*string) *string {
	for _, value := range values {
		if value != nil && strings.TrimSpace(*value) != "" {
			return value
		}
	}
	return nil
}

func noticeID(sourceType string, sourceID string) string {
	return strings.ToLower(sourceType) + ":" + sourceID
}

func buildCode(prefix string, now time.Time) string {
	buf := make([]byte, 4)
	_, _ = rand.Read(buf)
	return fmt.Sprintf("%s-%d-%s", prefix, now.UnixNano(), hex.EncodeToString(buf))
}

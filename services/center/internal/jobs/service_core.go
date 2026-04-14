package jobs

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	apperrors "mare/services/center/internal/errors"
	jobdto "mare/shared/contracts/dto/job"
)

type Service struct {
	pool             *pgxpool.Pool
	now              func() time.Time
	broker           *Broker
	executors        map[string]ItemExecutor
	externalTasks    ExternalTaskController
	issueSync        IssueSynchronizer
	notificationSync NotificationSynchronizer
	runningItems     map[string]context.CancelFunc
	runningItemsMu   sync.Mutex
	wakeCh           chan struct{}
	startOnce        sync.Once
	backgroundCtx    context.Context
	retryBaseDelay   time.Duration
	retryMaxAttempts int
}

func NewService(pool *pgxpool.Pool) *Service {
	return &Service{
		pool:         pool,
		now:          time.Now,
		broker:       NewBroker(),
		executors:    make(map[string]ItemExecutor),
		runningItems: make(map[string]context.CancelFunc),
		wakeCh:       make(chan struct{}, 1),
		retryBaseDelay:   30 * time.Second,
		retryMaxAttempts: 3,
	}
}

func (s *Service) SetRetryBaseDelay(delay time.Duration) {
	if delay > 0 {
		s.retryBaseDelay = delay
	}
}

func (s *Service) Start(ctx context.Context) {
	s.startOnce.Do(func() {
		s.backgroundCtx = ctx
		go s.run(ctx)
	})
}

func (s *Service) Broker() *Broker {
	return s.broker
}

func (s *Service) Subscribe(jobID string) (<-chan jobdto.StreamEvent, func()) {
	return s.broker.Subscribe(jobID)
}

func (s *Service) RegisterExecutor(jobIntent string, executor ItemExecutor) {
	s.executors[jobIntent] = executor
}

func (s *Service) SetExternalTaskController(controller ExternalTaskController) {
	s.externalTasks = controller
}

func (s *Service) SetIssueSynchronizer(sync IssueSynchronizer) {
	s.issueSync = sync
}

func (s *Service) SetNotificationSynchronizer(sync NotificationSynchronizer) {
	s.notificationSync = sync
}

func (s *Service) registerRunningItem(itemID string, cancel context.CancelFunc) {
	s.runningItemsMu.Lock()
	defer s.runningItemsMu.Unlock()
	s.runningItems[itemID] = cancel
}

func (s *Service) clearRunningItem(itemID string) {
	s.runningItemsMu.Lock()
	defer s.runningItemsMu.Unlock()
	delete(s.runningItems, itemID)
}

func (s *Service) interruptRunningItem(itemID string) bool {
	s.runningItemsMu.Lock()
	cancel, ok := s.runningItems[itemID]
	if ok {
		delete(s.runningItems, itemID)
	}
	s.runningItemsMu.Unlock()
	if ok {
		cancel()
	}
	return ok
}

func (s *Service) wake() {
	select {
	case s.wakeCh <- struct{}{}:
	default:
	}
}

func (s *Service) run(ctx context.Context) {
	_ = s.recoverQueuedJobs(ctx)
	s.wake()

	ticker := time.NewTicker(750 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		case <-s.wakeCh:
		}

		for {
			job, err := s.claimNextJob(ctx)
			if err != nil || job == nil {
				break
			}
			s.executeJob(ctx, *job)
		}
	}
}

func (s *Service) CreateJob(ctx context.Context, input CreateJobInput) (jobdto.CreateResponse, error) {
	if err := validateCreateJobInput(input); err != nil {
		return jobdto.CreateResponse{}, err
	}

	now := s.now().UTC()
	jobID := buildCode("job-id", now)
	jobCode := buildCode("job", now)

	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return jobdto.CreateResponse{}, err
	}
	defer tx.Rollback(ctx)

	var sourceSnapshot any
	if len(input.SourceSnapshot) > 0 {
		sourceSnapshot = input.SourceSnapshot
	}
	sourceSnapshotJSON, err := marshalNullableJSON(sourceSnapshot)
	if err != nil {
		return jobdto.CreateResponse{}, apperrors.BadRequest("作业来源快照序列化失败")
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO jobs (
			id, code, library_id, job_family, job_intent, route_type, status, priority,
			title, summary, source_domain, source_ref_id, source_snapshot,
			progress_percent, total_items, success_items, failed_items, skipped_items, issue_count,
			created_by_type, created_by_ref, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8,
			$9, $10, $11, $12, $13,
			0, $14, 0, 0, 0, 0,
			$15, $16, $17, $17
		)
	`, jobID, jobCode, input.LibraryID, input.JobFamily, input.JobIntent, input.RouteType, StatusPending, input.Priority,
		input.Title, strings.TrimSpace(input.Summary), input.SourceDomain, input.SourceRefID, sourceSnapshotJSON,
		len(input.Items), input.CreatedByType, input.CreatedByRef, now)
	if err != nil {
		return jobdto.CreateResponse{}, err
	}

	for _, link := range input.Links {
		if err := insertObjectLink(ctx, tx, jobID, nil, link, now); err != nil {
			return jobdto.CreateResponse{}, err
		}
	}

	for _, item := range input.Items {
		itemID := buildCode("job-item-id", now)
		_, err = tx.Exec(ctx, `
			INSERT INTO job_items (
				id, job_id, item_key, item_type, route_type, status, title, summary,
				source_path, target_path, progress_percent, attempt_count, issue_count,
				created_at, updated_at
			) VALUES (
				$1, $2, $3, $4, $5, $6, $7, $8,
				$9, $10, 0, 0, 0,
				$11, $11
			)
		`, itemID, jobID, item.ItemKey, item.ItemType, item.RouteType, ItemStatusPending, item.Title, strings.TrimSpace(item.Summary),
			item.SourcePath, item.TargetPath, now)
		if err != nil {
			return jobdto.CreateResponse{}, err
		}
		for _, link := range item.Links {
			if err := insertObjectLink(ctx, tx, jobID, &itemID, link, now); err != nil {
				return jobdto.CreateResponse{}, err
			}
		}
	}

	event, err := s.insertEvent(ctx, tx, eventInsertInput{
		JobID:     jobID,
		EventType: EventJobCreated,
		Message:   fmt.Sprintf("作业已创建：%s", input.Title),
		JobStatus: ptr(StatusPending),
		Payload: map[string]any{
			"jobIntent":    input.JobIntent,
			"sourceDomain": input.SourceDomain,
			"totalItems":   len(input.Items),
		},
		CreatedAt: now,
	})
	if err != nil {
		return jobdto.CreateResponse{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return jobdto.CreateResponse{}, err
	}

	s.publish(event)
	record, err := s.loadJobRecord(ctx, jobID)
	if err != nil {
		return jobdto.CreateResponse{}, err
	}

	s.wake()
	return jobdto.CreateResponse{
		Message: "作业已创建",
		JobID:   jobID,
		Job:     record,
	}, nil
}

func (s *Service) ListJobs(ctx context.Context, query ListQuery) (jobdto.ListResponse, error) {
	page := query.Page
	if page <= 0 {
		page = 1
	}
	pageSize := query.PageSize
	if pageSize <= 0 {
		pageSize = 20
	}

	whereParts := []string{"TRUE"}
	args := make([]any, 0)
	addArg := func(value any) string {
		args = append(args, value)
		return fmt.Sprintf("$%d", len(args))
	}

	if text := strings.TrimSpace(query.SearchText); text != "" {
		placeholder := addArg("%" + text + "%")
		whereParts = append(whereParts, fmt.Sprintf("(title ILIKE %s OR summary ILIKE %s)", placeholder, placeholder))
	}
	if status := strings.TrimSpace(query.Status); status != "" {
		whereParts = append(whereParts, fmt.Sprintf("status = %s", addArg(status)))
	}
	if family := strings.TrimSpace(query.JobFamily); family != "" {
		whereParts = append(whereParts, fmt.Sprintf("job_family = %s", addArg(family)))
	}
	if sourceDomain := strings.TrimSpace(query.SourceDomain); sourceDomain != "" {
		whereParts = append(whereParts, fmt.Sprintf("source_domain = %s", addArg(sourceDomain)))
	}
	if libraryID := strings.TrimSpace(query.LibraryID); libraryID != "" {
		whereParts = append(whereParts, fmt.Sprintf("library_id = %s", addArg(libraryID)))
	}

	whereSQL := strings.Join(whereParts, " AND ")
	var total int
	if err := s.pool.QueryRow(ctx, "SELECT COUNT(*) FROM jobs WHERE "+whereSQL, args...).Scan(&total); err != nil {
		return jobdto.ListResponse{}, err
	}

	args = append(args, pageSize, (page-1)*pageSize)
	rows, err := s.pool.Query(ctx, `
		SELECT
			id, code, library_id, job_family, job_intent, route_type, status, priority,
			title, summary, source_domain, source_ref_id, source_snapshot,
			progress_percent, speed_bps, eta_seconds, total_items, success_items, failed_items, skipped_items,
			issue_count, latest_error_code, latest_error_message, outcome_summary,
			created_by_type, created_by_ref, created_at, started_at, finished_at, canceled_at, updated_at
		FROM jobs
		WHERE `+whereSQL+`
		ORDER BY updated_at DESC, created_at DESC
		LIMIT $`+fmt.Sprint(len(args)-1)+` OFFSET $`+fmt.Sprint(len(args))+`
	`, args...)
	if err != nil {
		return jobdto.ListResponse{}, err
	}
	defer rows.Close()

	items := make([]jobdto.Record, 0)
	for rows.Next() {
		row, err := scanJobRow(rows)
		if err != nil {
			return jobdto.ListResponse{}, err
		}
		items = append(items, mapJobRow(row))
	}
	if err := rows.Err(); err != nil {
		return jobdto.ListResponse{}, err
	}

	return jobdto.ListResponse{
		Items:    items,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	}, nil
}

func (s *Service) LoadJobDetail(ctx context.Context, id string) (jobdto.Detail, error) {
	job, err := s.loadJobRecord(ctx, id)
	if err != nil {
		return jobdto.Detail{}, err
	}
	items, err := s.loadJobItems(ctx, id)
	if err != nil {
		return jobdto.Detail{}, err
	}
	links, err := s.loadJobLinks(ctx, id)
	if err != nil {
		return jobdto.Detail{}, err
	}
	return jobdto.Detail{Job: job, Items: items, Links: links}, nil
}

func (s *Service) ListJobEvents(ctx context.Context, id string) (jobdto.EventListResponse, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, job_id, job_item_id, job_attempt_id, event_type, message, payload, created_at
		FROM job_events
		WHERE job_id = $1
		ORDER BY created_at DESC
	`, id)
	if err != nil {
		return jobdto.EventListResponse{}, err
	}
	defer rows.Close()

	items := make([]jobdto.EventRecord, 0)
	for rows.Next() {
		var row eventRow
		if err := rows.Scan(&row.ID, &row.JobID, &row.JobItemID, &row.JobAttemptID, &row.EventType, &row.Message, &row.Payload, &row.CreatedAt); err != nil {
			return jobdto.EventListResponse{}, err
		}
		items = append(items, mapEventRow(row))
	}
	if err := rows.Err(); err != nil {
		return jobdto.EventListResponse{}, err
	}

	return jobdto.EventListResponse{Items: items}, nil
}

package issues

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	apperrors "mare/services/center/internal/errors"
	issuedto "mare/shared/contracts/dto/issue"
	jobdto "mare/shared/contracts/dto/job"
)

const (
	CategoryConflict       = "CONFLICT"
	CategoryTransfer       = "TRANSFER"
	CategoryVerify         = "VERIFY"
	CategoryNodePermission = "NODE_PERMISSION"
	CategoryScanParse      = "SCAN_PARSE"
	CategoryCleanup        = "CLEANUP_GOVERNANCE"

	NatureBlocking = "BLOCKING"

	SourceDomainTransferJob    = "TRANSFER_JOB"
	SourceDomainMaintenanceJob = "MAINTENANCE_JOB"
	SourceDomainFileCenter     = "FILE_CENTER"
	SourceDomainStorage        = "STORAGE_DOMAIN"
	SourceDomainGovernance     = "SYSTEM_GOVERNANCE"
	SourceDomainImport         = "IMPORT_DOMAIN"

	SeverityCritical = "CRITICAL"
	SeverityWarning  = "WARNING"
	SeverityInfo     = "INFO"

	StatusOpen                 = "OPEN"
	StatusAwaitingConfirmation = "AWAITING_CONFIRMATION"
	StatusInProgress           = "IN_PROGRESS"
	StatusIgnored              = "IGNORED"
	StatusResolved             = "RESOLVED"
	StatusArchived             = "ARCHIVED"

	ActionRetry       = "retry"
	ActionConfirm     = "confirm"
	ActionIgnore      = "ignore"
	ActionArchive     = "archive"
	EventDetected     = "DETECTED"
	EventUpdated      = "UPDATED"
	EventRetry        = "RETRY_REQUESTED"
	EventConfirmed    = "CONFIRMED"
	EventIgnored      = "IGNORED"
	EventResolved     = "RESOLVED"
	EventArchived     = "ARCHIVED"
	EventAutoReopened = "AUTO_REOPENED"
)

type JobController interface {
	RetryJob(ctx context.Context, id string) (jobdto.MutationResponse, error)
}

type ListQuery struct {
	Page          int
	PageSize      int
	SearchText    string
	IssueCategory string
	SourceDomain  string
	LibraryID     string
	Status        string
	Severity      string
	Nature        string
	SortValue     string
	EndpointID    string
	Path          string
	JobIDs        []string
}

type ActionRequest = issuedto.ActionRequest

type Service struct {
	pool             *pgxpool.Pool
	now              func() time.Time
	jobController    JobController
	notificationSync interface {
		SyncJobNotifications(ctx context.Context, jobID string) error
	}
}

func NewService(pool *pgxpool.Pool, jobController JobController) *Service {
	return &Service{
		pool:          pool,
		now:           time.Now,
		jobController: jobController,
	}
}

func (s *Service) SetJobController(jobController JobController) {
	s.jobController = jobController
}

func (s *Service) SetNotificationSynchronizer(sync interface {
	SyncJobNotifications(ctx context.Context, jobID string) error
}) {
	s.notificationSync = sync
}

func (s *Service) ListIssues(ctx context.Context, query ListQuery) (issuedto.ListResponse, error) {
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
		whereParts = append(whereParts, fmt.Sprintf(`(
			title ILIKE %s OR
			summary ILIKE %s OR
			issue_type ILIKE %s OR
			object_label ILIKE %s OR
			COALESCE(asset_label, '') ILIKE %s OR
			COALESCE(detail, '') ILIKE %s OR
			COALESCE(source_snapshot ->> 'taskTitle', '') ILIKE %s OR
			COALESCE(source_snapshot ->> 'taskItemTitle', '') ILIKE %s OR
			COALESCE(source_snapshot ->> 'endpointLabel', '') ILIKE %s OR
			COALESCE(source_snapshot ->> 'path', '') ILIKE %s OR
			COALESCE(source_snapshot ->> 'sourceLabel', '') ILIKE %s
		)`, placeholder, placeholder, placeholder, placeholder, placeholder, placeholder, placeholder, placeholder, placeholder, placeholder, placeholder))
	}
	if category := strings.TrimSpace(query.IssueCategory); category != "" {
		whereParts = append(whereParts, fmt.Sprintf("issue_category = %s", addArg(category)))
	}
	if sourceDomain := strings.TrimSpace(query.SourceDomain); sourceDomain != "" {
		whereParts = append(whereParts, fmt.Sprintf("source_domain = %s", addArg(sourceDomain)))
	}
	if libraryID := strings.TrimSpace(query.LibraryID); libraryID != "" {
		whereParts = append(whereParts, fmt.Sprintf("library_id = %s", addArg(libraryID)))
	}
	if status := strings.TrimSpace(query.Status); status != "" {
		whereParts = append(whereParts, fmt.Sprintf("status = %s", addArg(status)))
	}
	if severity := strings.TrimSpace(query.Severity); severity != "" {
		whereParts = append(whereParts, fmt.Sprintf("severity = %s", addArg(severity)))
	}
	if nature := strings.TrimSpace(query.Nature); nature != "" {
		whereParts = append(whereParts, fmt.Sprintf("nature = %s", addArg(nature)))
	}
	if endpointID := strings.TrimSpace(query.EndpointID); endpointID != "" {
		whereParts = append(whereParts, fmt.Sprintf("COALESCE(source_snapshot ->> 'endpointId', '') = %s", addArg(endpointID)))
	}
	if path := strings.TrimSpace(query.Path); path != "" {
		whereParts = append(whereParts, fmt.Sprintf("COALESCE(source_snapshot ->> 'path', '') ILIKE %s", addArg("%"+path+"%")))
	}
	if len(query.JobIDs) > 0 {
		whereParts = append(whereParts, fmt.Sprintf("COALESCE(source_snapshot ->> 'taskId', '') = ANY(%s)", addArg(query.JobIDs)))
	}

	whereSQL := strings.Join(whereParts, " AND ")
	var total int
	if err := s.pool.QueryRow(ctx, "SELECT COUNT(*) FROM issues WHERE "+whereSQL, args...).Scan(&total); err != nil {
		return issuedto.ListResponse{}, err
	}

	sortSQL := "updated_at DESC, created_at DESC"
	switch query.SortValue {
	case "SEVERITY":
		sortSQL = "CASE severity WHEN 'CRITICAL' THEN 0 WHEN 'WARNING' THEN 1 ELSE 2 END, updated_at DESC"
	case "TITLE":
		sortSQL = "title ASC, updated_at DESC"
	}

	args = append(args, pageSize, (page-1)*pageSize)
	rows, err := s.pool.Query(ctx, `
		SELECT
			id, code, library_id, issue_category, issue_type, nature, source_domain, severity, status, dedupe_key,
			title, summary, object_label, asset_label, suggested_action, suggested_action_label, suggestion, detail, occurrence_count, last_detection_key,
			source_snapshot, impact_snapshot, first_detected_at, last_detected_at, last_status_changed_at,
			resolved_at, archived_at, latest_event_at, latest_error_code, latest_error_message, created_at, updated_at
		FROM issues
		WHERE `+whereSQL+`
		ORDER BY `+sortSQL+`
		LIMIT $`+fmt.Sprint(len(args)-1)+` OFFSET $`+fmt.Sprint(len(args))+`
	`, args...)
	if err != nil {
		return issuedto.ListResponse{}, err
	}
	defer rows.Close()

	items := make([]issuedto.Record, 0)
	for rows.Next() {
		row, err := scanIssueRow(rows)
		if err != nil {
			return issuedto.ListResponse{}, err
		}
		histories, err := s.loadHistories(ctx, row.ID)
		if err != nil {
			return issuedto.ListResponse{}, err
		}
		items = append(items, mapIssueRow(row, histories))
	}
	if err := rows.Err(); err != nil {
		return issuedto.ListResponse{}, err
	}

	return issuedto.ListResponse{
		Items:    items,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	}, nil
}

func (s *Service) ListByJobIDs(ctx context.Context, jobIDs []string) ([]issuedto.Record, error) {
	if len(jobIDs) == 0 {
		return []issuedto.Record{}, nil
	}
	response, err := s.ListIssues(ctx, ListQuery{
		Page:     1,
		PageSize: max(len(jobIDs)*8, 50),
		JobIDs:   jobIDs,
	})
	if err != nil {
		return nil, err
	}
	return response.Items, nil
}

func (s *Service) ApplyAction(ctx context.Context, request ActionRequest) (issuedto.ActionResponse, error) {
	if len(request.IDs) == 0 {
		return issuedto.ActionResponse{}, apperrors.BadRequest("缺少异常 ID")
	}

	switch request.Action {
	case ActionRetry:
		jobIDs, err := s.transitionIssues(ctx, request.IDs, StatusInProgress, EventRetry, "异常中心已发起重试，等待任务结果回写。")
		if err != nil {
			return issuedto.ActionResponse{}, err
		}
		if s.jobController == nil {
			return issuedto.ActionResponse{}, apperrors.BadRequest("当前异常未接入任务重试控制器")
		}
		for _, jobID := range jobIDs {
			if _, err := s.jobController.RetryJob(ctx, jobID); err != nil {
				return issuedto.ActionResponse{}, err
			}
		}
		return issuedto.ActionResponse{Message: "异常已进入重试流程", IDs: request.IDs}, nil
	case ActionConfirm:
		if _, err := s.transitionIssues(ctx, request.IDs, StatusResolved, EventConfirmed, "已确认当前异常，主工作区不再继续提醒。"); err != nil {
			return issuedto.ActionResponse{}, err
		}
		return issuedto.ActionResponse{Message: "异常已标记为已确认", IDs: request.IDs}, nil
	case ActionIgnore:
		if _, err := s.transitionIssues(ctx, request.IDs, StatusIgnored, EventIgnored, "已忽略当前异常，但历史记录仍保留。"); err != nil {
			return issuedto.ActionResponse{}, err
		}
		return issuedto.ActionResponse{Message: "异常已忽略", IDs: request.IDs}, nil
	case ActionArchive:
		if _, err := s.transitionIssues(ctx, request.IDs, StatusArchived, EventArchived, "已归档当前异常，可在历史中回查。"); err != nil {
			return issuedto.ActionResponse{}, err
		}
		return issuedto.ActionResponse{Message: "异常已归档", IDs: request.IDs}, nil
	default:
		return issuedto.ActionResponse{}, apperrors.BadRequest("不支持的异常动作")
	}
}

func (s *Service) ClearHistory(ctx context.Context, request issuedto.ClearHistoryRequest) (issuedto.ClearHistoryResponse, error) {
	if len(request.IDs) == 0 {
		return issuedto.ClearHistoryResponse{}, apperrors.BadRequest("缺少异常 ID")
	}

	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return issuedto.ClearHistoryResponse{}, err
	}
	defer tx.Rollback(ctx)

	rows, err := tx.Query(ctx, `
		DELETE FROM issues
		WHERE id = ANY($1)
		  AND status IN ('IGNORED', 'RESOLVED', 'ARCHIVED')
		RETURNING COALESCE(source_snapshot ->> 'taskId', '')
	`, request.IDs)
	if err != nil {
		return issuedto.ClearHistoryResponse{}, err
	}
	defer rows.Close()

	jobIDs := make(map[string]struct{})
	for rows.Next() {
		var jobID string
		if err := rows.Scan(&jobID); err != nil {
			return issuedto.ClearHistoryResponse{}, err
		}
		if strings.TrimSpace(jobID) != "" {
			jobIDs[jobID] = struct{}{}
		}
	}
	if err := rows.Err(); err != nil {
		return issuedto.ClearHistoryResponse{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return issuedto.ClearHistoryResponse{}, err
	}

	for jobID := range jobIDs {
		if err := s.refreshJobIssueCounters(ctx, jobID); err != nil {
			return issuedto.ClearHistoryResponse{}, err
		}
		s.syncJobNotifications(ctx, jobID)
	}
	return issuedto.ClearHistoryResponse{Message: "历史异常已清理", IDs: request.IDs}, nil
}

func (s *Service) syncJobNotifications(ctx context.Context, jobID string) {
	if s.notificationSync == nil || strings.TrimSpace(jobID) == "" {
		return
	}
	_ = s.notificationSync.SyncJobNotifications(ctx, jobID)
}

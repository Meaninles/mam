package jobs

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	apperrors "mare/services/center/internal/errors"
	jobdto "mare/shared/contracts/dto/job"
)

func (s *Service) loadJobRecord(ctx context.Context, id string) (jobdto.Record, error) {
	row, err := s.loadJobRow(ctx, id)
	if err != nil {
		return jobdto.Record{}, err
	}
	return mapJobRow(row), nil
}

func (s *Service) loadJobRow(ctx context.Context, id string) (jobRow, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT
			id, code, library_id, job_family, job_intent, route_type, status, priority,
			title, summary, source_domain, source_ref_id, source_snapshot,
			progress_percent, speed_bps, eta_seconds, total_items, success_items, failed_items, skipped_items,
			issue_count, latest_error_code, latest_error_message, outcome_summary,
			created_by_type, created_by_ref, created_at, started_at, finished_at, canceled_at, updated_at
		FROM jobs
		WHERE id = $1
	`, id)
	return scanJobRow(row)
}

func (s *Service) loadJobRowTx(ctx context.Context, tx pgx.Tx, id string) (jobRow, error) {
	row := tx.QueryRow(ctx, `
		SELECT
			id, code, library_id, job_family, job_intent, route_type, status, priority,
			title, summary, source_domain, source_ref_id, source_snapshot,
			progress_percent, speed_bps, eta_seconds, total_items, success_items, failed_items, skipped_items,
			issue_count, latest_error_code, latest_error_message, outcome_summary,
			created_by_type, created_by_ref, created_at, started_at, finished_at, canceled_at, updated_at
		FROM jobs
		WHERE id = $1
		FOR UPDATE
	`, id)
	return scanJobRow(row)
}

func (s *Service) loadJobItems(ctx context.Context, jobID string) ([]jobdto.ItemRecord, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT
			id, job_id, parent_item_id, item_key, item_type, route_type, status, phase, title, summary,
			source_path, target_path, progress_percent, speed_bps, eta_seconds, bytes_total, bytes_done,
			attempt_count, issue_count, latest_error_code, latest_error_message, result_summary,
			started_at, finished_at, canceled_at, updated_at, created_at
		FROM job_items
		WHERE job_id = $1
		ORDER BY created_at ASC
	`, jobID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]jobdto.ItemRecord, 0)
	for rows.Next() {
		row, err := scanItemRow(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, mapItemRow(row))
	}
	return items, rows.Err()
}

func (s *Service) loadItemRows(ctx context.Context, jobID string) ([]itemRow, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT
			id, job_id, parent_item_id, item_key, item_type, route_type, status, phase, title, summary,
			source_path, target_path, progress_percent, speed_bps, eta_seconds, bytes_total, bytes_done,
			attempt_count, issue_count, latest_error_code, latest_error_message, result_summary,
			started_at, finished_at, canceled_at, updated_at, created_at
		FROM job_items
		WHERE job_id = $1
		ORDER BY created_at ASC
	`, jobID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]itemRow, 0)
	for rows.Next() {
		row, err := scanItemRow(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, row)
	}
	return items, rows.Err()
}

func (s *Service) loadJobItemRecord(ctx context.Context, itemID string) (jobdto.ItemRecord, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT
			id, job_id, parent_item_id, item_key, item_type, route_type, status, phase, title, summary,
			source_path, target_path, progress_percent, speed_bps, eta_seconds, bytes_total, bytes_done,
			attempt_count, issue_count, latest_error_code, latest_error_message, result_summary,
			started_at, finished_at, canceled_at, updated_at, created_at
		FROM job_items
		WHERE id = $1
	`, itemID)
	item, err := scanItemRow(row)
	if err != nil {
		return jobdto.ItemRecord{}, err
	}
	return mapItemRow(item), nil
}

func (s *Service) loadItemRowTx(ctx context.Context, tx pgx.Tx, itemID string) (itemRow, error) {
	row := tx.QueryRow(ctx, `
		SELECT
			id, job_id, parent_item_id, item_key, item_type, route_type, status, phase, title, summary,
			source_path, target_path, progress_percent, speed_bps, eta_seconds, bytes_total, bytes_done,
			attempt_count, issue_count, latest_error_code, latest_error_message, result_summary,
			started_at, finished_at, canceled_at, updated_at, created_at
		FROM job_items
		WHERE id = $1
		FOR UPDATE
	`, itemID)
	return scanItemRow(row)
}

func (s *Service) loadJobLinks(ctx context.Context, jobID string) ([]jobdto.ObjectLinkRecord, error) {
	return s.loadLinks(ctx, "job_id = $1 AND job_item_id IS NULL", jobID)
}

func (s *Service) loadItemLinks(ctx context.Context, itemID string) ([]jobdto.ObjectLinkRecord, error) {
	return s.loadLinks(ctx, "job_item_id = $1", itemID)
}

func (s *Service) loadLinks(ctx context.Context, condition string, arg string) ([]jobdto.ObjectLinkRecord, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, job_id, job_item_id, link_role, object_type, asset_id, asset_replica_id, directory_id, mount_id, storage_node_id, created_at
		FROM job_object_links
		WHERE `+condition+`
		ORDER BY created_at ASC
	`, arg)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	links := make([]jobdto.ObjectLinkRecord, 0)
	for rows.Next() {
		var link jobdto.ObjectLinkRecord
		var createdAt time.Time
		if err := rows.Scan(&link.ID, &link.JobID, &link.JobItemID, &link.LinkRole, &link.ObjectType, &link.AssetID, &link.AssetReplicaID, &link.DirectoryID, &link.MountID, &link.StorageNodeID, &createdAt); err != nil {
			return nil, err
		}
		link.CreatedAt = createdAt.UTC().Format(time.RFC3339)
		links = append(links, link)
	}
	return links, rows.Err()
}

type eventInsertInput struct {
	JobID      string
	JobItemID  *string
	AttemptID  *string
	EventType  string
	Message    string
	JobStatus  *string
	ItemStatus *string
	Payload    map[string]any
	CreatedAt  time.Time
}

func (s *Service) insertEvent(ctx context.Context, tx pgx.Tx, input eventInsertInput) (jobdto.StreamEvent, error) {
	payloadJSON, err := marshalNullableJSON(input.Payload)
	if err != nil {
		return jobdto.StreamEvent{}, err
	}
	eventID := buildCode("job-event-id", input.CreatedAt)
	_, err = tx.Exec(ctx, `
		INSERT INTO job_events (
			id, job_id, job_item_id, job_attempt_id, event_type, message, payload, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`, eventID, input.JobID, input.JobItemID, input.AttemptID, input.EventType, input.Message, payloadJSON, input.CreatedAt)
	if err != nil {
		return jobdto.StreamEvent{}, err
	}
	return jobdto.StreamEvent{
		EventID:    eventID,
		Topic:      "JOB",
		EventType:  input.EventType,
		JobID:      input.JobID,
		JobItemID:  input.JobItemID,
		JobStatus:  input.JobStatus,
		ItemStatus: input.ItemStatus,
		Message:    input.Message,
		CreatedAt:  input.CreatedAt.Format(time.RFC3339),
	}, nil
}

func (s *Service) createJobAttempt(ctx context.Context, jobID string, itemID *string, status string, errorCode *string, errorMessage *string) error {
	now := s.now().UTC()
	var nextAttempt int
	err := s.pool.QueryRow(ctx, `
		SELECT COALESCE(MAX(attempt_no), 0) + 1
		FROM job_attempts
		WHERE job_id = $1
		  AND job_item_id IS NULL
	`, jobID).Scan(&nextAttempt)
	if err != nil {
		return err
	}
	_, err = s.pool.Exec(ctx, `
		INSERT INTO job_attempts (
			id, job_id, job_item_id, attempt_no, status, worker_type, error_code, error_message, started_at, finished_at
		) VALUES ($1, $2, $3, $4, $5, 'CENTER', $6, $7, $8, CASE WHEN $5 = 'RUNNING' THEN NULL ELSE $8 END)
	`, buildCode("job-attempt-id", now), jobID, itemID, nextAttempt, status, errorCode, errorMessage, now)
	return err
}

func (s *Service) publish(event jobdto.StreamEvent) {
	s.broker.Publish(event)
}

func (s *Service) refreshJobAggregate(ctx context.Context, jobID string, now time.Time) error {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err := s.refreshJobAggregateTx(ctx, tx, jobID, now); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *Service) refreshJobAggregateTx(ctx context.Context, tx pgx.Tx, jobID string, now time.Time) error {
	var avgProgress float64
	var successItems int
	var failedItems int
	var skippedItems int
	err := tx.QueryRow(ctx, `
		SELECT
			COALESCE(AVG(progress_percent), 0),
			COUNT(*) FILTER (WHERE status = 'COMPLETED'),
			COUNT(*) FILTER (WHERE status = 'FAILED'),
			COUNT(*) FILTER (WHERE status = 'SKIPPED')
		FROM job_items
		WHERE job_id = $1
	`, jobID).Scan(&avgProgress, &successItems, &failedItems, &skippedItems)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `
		UPDATE jobs
		SET progress_percent = $2,
		    success_items = $3,
		    failed_items = $4,
		    skipped_items = $5,
		    updated_at = $6
		WHERE id = $1
	`, jobID, avgProgress, successItems, failedItems, skippedItems, now)
	return err
}

func scanJobRow(row pgx.Row) (jobRow, error) {
	var item jobRow
	err := row.Scan(
		&item.ID,
		&item.Code,
		&item.LibraryID,
		&item.JobFamily,
		&item.JobIntent,
		&item.RouteType,
		&item.Status,
		&item.Priority,
		&item.Title,
		&item.Summary,
		&item.SourceDomain,
		&item.SourceRefID,
		&item.SourceSnapshot,
		&item.ProgressPercent,
		&item.SpeedBPS,
		&item.ETASeconds,
		&item.TotalItems,
		&item.SuccessItems,
		&item.FailedItems,
		&item.SkippedItems,
		&item.IssueCount,
		&item.LatestErrorCode,
		&item.LatestErrorMessage,
		&item.OutcomeSummary,
		&item.CreatedByType,
		&item.CreatedByRef,
		&item.CreatedAt,
		&item.StartedAt,
		&item.FinishedAt,
		&item.CanceledAt,
		&item.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return jobRow{}, apperrors.NotFound("作业不存在")
	}
	return item, err
}

func scanItemRow(row pgx.Row) (itemRow, error) {
	var item itemRow
	err := row.Scan(
		&item.ID,
		&item.JobID,
		&item.ParentItemID,
		&item.ItemKey,
		&item.ItemType,
		&item.RouteType,
		&item.Status,
		&item.Phase,
		&item.Title,
		&item.Summary,
		&item.SourcePath,
		&item.TargetPath,
		&item.ProgressPercent,
		&item.SpeedBPS,
		&item.ETASeconds,
		&item.BytesTotal,
		&item.BytesDone,
		&item.AttemptCount,
		&item.IssueCount,
		&item.LatestErrorCode,
		&item.LatestErrorMessage,
		&item.ResultSummary,
		&item.StartedAt,
		&item.FinishedAt,
		&item.CanceledAt,
		&item.UpdatedAt,
		&item.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return itemRow{}, apperrors.NotFound("作业子项不存在")
	}
	return item, err
}

func mapJobRow(row jobRow) jobdto.Record {
	var snapshot any
	if len(row.SourceSnapshot) > 0 {
		_ = json.Unmarshal(row.SourceSnapshot, &snapshot)
	}
	return jobdto.Record{
		ID:                 row.ID,
		Code:               row.Code,
		LibraryID:          row.LibraryID,
		JobFamily:          row.JobFamily,
		JobIntent:          row.JobIntent,
		RouteType:          row.RouteType,
		Status:             row.Status,
		Priority:           row.Priority,
		Title:              row.Title,
		Summary:            row.Summary,
		SourceDomain:       row.SourceDomain,
		SourceRefID:        row.SourceRefID,
		SourceSnapshot:     snapshot,
		ProgressPercent:    row.ProgressPercent,
		SpeedBPS:           row.SpeedBPS,
		ETASeconds:         row.ETASeconds,
		TotalItems:         row.TotalItems,
		SuccessItems:       row.SuccessItems,
		FailedItems:        row.FailedItems,
		SkippedItems:       row.SkippedItems,
		IssueCount:         row.IssueCount,
		LatestErrorCode:    row.LatestErrorCode,
		LatestErrorMessage: row.LatestErrorMessage,
		OutcomeSummary:     row.OutcomeSummary,
		CreatedByType:      row.CreatedByType,
		CreatedByRef:       row.CreatedByRef,
		CreatedAt:          row.CreatedAt.UTC().Format(time.RFC3339),
		StartedAt:          formatOptionalRFC3339(row.StartedAt),
		FinishedAt:         formatOptionalRFC3339(row.FinishedAt),
		CanceledAt:         formatOptionalRFC3339(row.CanceledAt),
		UpdatedAt:          row.UpdatedAt.UTC().Format(time.RFC3339),
	}
}

func mapItemRow(row itemRow) jobdto.ItemRecord {
	return jobdto.ItemRecord{
		ID:                 row.ID,
		JobID:              row.JobID,
		ParentItemID:       row.ParentItemID,
		ItemKey:            row.ItemKey,
		ItemType:           row.ItemType,
		RouteType:          row.RouteType,
		Status:             row.Status,
		Phase:              row.Phase,
		Title:              row.Title,
		Summary:            row.Summary,
		SourcePath:         row.SourcePath,
		TargetPath:         row.TargetPath,
		ProgressPercent:    row.ProgressPercent,
		SpeedBPS:           row.SpeedBPS,
		ETASeconds:         row.ETASeconds,
		BytesTotal:         row.BytesTotal,
		BytesDone:          row.BytesDone,
		AttemptCount:       row.AttemptCount,
		IssueCount:         row.IssueCount,
		LatestErrorCode:    row.LatestErrorCode,
		LatestErrorMessage: row.LatestErrorMessage,
		ResultSummary:      row.ResultSummary,
		StartedAt:          formatOptionalRFC3339(row.StartedAt),
		FinishedAt:         formatOptionalRFC3339(row.FinishedAt),
		CanceledAt:         formatOptionalRFC3339(row.CanceledAt),
		UpdatedAt:          row.UpdatedAt.UTC().Format(time.RFC3339),
		CreatedAt:          row.CreatedAt.UTC().Format(time.RFC3339),
	}
}

func mapEventRow(row eventRow) jobdto.EventRecord {
	var payload any
	if len(row.Payload) > 0 {
		_ = json.Unmarshal(row.Payload, &payload)
	}
	return jobdto.EventRecord{
		ID:           row.ID,
		JobID:        row.JobID,
		JobItemID:    row.JobItemID,
		JobAttemptID: row.JobAttemptID,
		EventType:    row.EventType,
		Message:      row.Message,
		Payload:      payload,
		CreatedAt:    row.CreatedAt.UTC().Format(time.RFC3339),
	}
}

func insertObjectLink(ctx context.Context, tx pgx.Tx, jobID string, jobItemID *string, link CreateObjectLinkInput, now time.Time) error {
	if !isValidObjectLink(link) {
		return apperrors.BadRequest("作业关联对象无效")
	}
	_, err := tx.Exec(ctx, `
		INSERT INTO job_object_links (
			id, job_id, job_item_id, link_role, object_type, asset_id, asset_replica_id, directory_id, mount_id, storage_node_id, created_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11
		)
	`, buildCode("job-link-id", now), jobID, jobItemID, link.LinkRole, link.ObjectType, link.AssetID, link.AssetReplicaID, link.DirectoryID, link.MountID, link.StorageNodeID, now)
	return err
}

func validateCreateJobInput(input CreateJobInput) error {
	if len(input.Items) == 0 {
		return apperrors.BadRequest("作业至少需要一个子项")
	}
	if strings.TrimSpace(input.JobFamily) == "" || strings.TrimSpace(input.JobIntent) == "" {
		return apperrors.BadRequest("作业类型不能为空")
	}
	if strings.TrimSpace(input.Title) == "" {
		return apperrors.BadRequest("作业标题不能为空")
	}
	if strings.TrimSpace(input.SourceDomain) == "" {
		return apperrors.BadRequest("来源域不能为空")
	}
	for _, item := range input.Items {
		if strings.TrimSpace(item.ItemKey) == "" || strings.TrimSpace(item.ItemType) == "" || strings.TrimSpace(item.Title) == "" {
			return apperrors.BadRequest("作业子项缺少必要字段")
		}
	}
	return nil
}

func isValidObjectLink(link CreateObjectLinkInput) bool {
	count := 0
	for _, value := range [](*string){link.AssetID, link.AssetReplicaID, link.DirectoryID, link.MountID, link.StorageNodeID} {
		if value != nil && strings.TrimSpace(*value) != "" {
			count++
		}
	}
	return count == 1 && strings.TrimSpace(link.LinkRole) != "" && strings.TrimSpace(link.ObjectType) != ""
}

func containsStatus(allowed []string, current string) bool {
	for _, item := range allowed {
		if item == current {
			return true
		}
	}
	return false
}

func marshalNullableJSON(value any) ([]byte, error) {
	if value == nil {
		return nil, nil
	}
	return json.Marshal(value)
}

func formatOptionalRFC3339(value *time.Time) *string {
	if value == nil {
		return nil
	}
	formatted := value.UTC().Format(time.RFC3339)
	return &formatted
}

func buildCode(prefix string, now time.Time) string {
	buf := make([]byte, 4)
	_, _ = rand.Read(buf)
	return fmt.Sprintf("%s-%d-%s", prefix, now.UnixNano(), hex.EncodeToString(buf))
}

func ptr[T any](value T) *T {
	return &value
}

package jobs

import (
	"context"
	"github.com/jackc/pgx/v5"

	apperrors "mare/services/center/internal/errors"
	jobdto "mare/shared/contracts/dto/job"
)

func (s *Service) PauseJob(ctx context.Context, id string) (jobdto.MutationResponse, error) {
	return s.transitionJob(ctx, id, []string{StatusPending, StatusQueued, StatusRunning, StatusWaitingRetry}, StatusPaused, EventJobPaused, "作业已暂停", nil)
}

func (s *Service) ResumeJob(ctx context.Context, id string) (jobdto.MutationResponse, error) {
	result, err := s.transitionJob(ctx, id, []string{StatusPaused}, StatusQueued, EventJobResumed, "作业已恢复排队", nil)
	if err == nil {
		s.wake()
	}
	return result, err
}

func (s *Service) CancelJob(ctx context.Context, id string) (jobdto.MutationResponse, error) {
	now := s.now().UTC()
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return jobdto.MutationResponse{}, err
	}
	defer tx.Rollback(ctx)

	current, err := s.loadJobRowTx(ctx, tx, id)
	if err != nil {
		return jobdto.MutationResponse{}, err
	}
	if !containsStatus([]string{StatusPending, StatusQueued, StatusRunning, StatusWaitingRetry, StatusPaused}, current.Status) {
		return jobdto.MutationResponse{}, apperrors.BadRequest("当前状态不允许取消")
	}

	_, err = tx.Exec(ctx, `
		UPDATE jobs
		SET status = $2,
		    canceled_at = $3,
		    updated_at = $3
		WHERE id = $1
	`, id, StatusCanceled, now)
	if err != nil {
		return jobdto.MutationResponse{}, err
	}

	if current.Status != StatusRunning {
		if _, err := tx.Exec(ctx, `
			UPDATE job_items
			SET status = CASE WHEN status = 'COMPLETED' THEN status ELSE 'CANCELED' END,
			    progress_percent = CASE WHEN status = 'COMPLETED' THEN progress_percent ELSE 100 END,
			    canceled_at = COALESCE(canceled_at, $2),
			    finished_at = COALESCE(finished_at, $2),
			    updated_at = $2
			WHERE job_id = $1
		`, id, now); err != nil {
			return jobdto.MutationResponse{}, err
		}
		if err := s.refreshJobAggregateTx(ctx, tx, id, now); err != nil {
			return jobdto.MutationResponse{}, err
		}
	}

	event, err := s.insertEvent(ctx, tx, eventInsertInput{
		JobID:     id,
		EventType: EventJobCanceled,
		Message:   "作业已取消",
		JobStatus: ptr(StatusCanceled),
		CreatedAt: now,
	})
	if err != nil {
		return jobdto.MutationResponse{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return jobdto.MutationResponse{}, err
	}

	s.publish(event)
	job, err := s.loadJobRecord(ctx, id)
	if err != nil {
		return jobdto.MutationResponse{}, err
	}
	return jobdto.MutationResponse{Message: "作业已取消", Job: job}, nil
}

func (s *Service) RetryJob(ctx context.Context, id string) (jobdto.MutationResponse, error) {
	now := s.now().UTC()
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return jobdto.MutationResponse{}, err
	}
	defer tx.Rollback(ctx)

	current, err := s.loadJobRowTx(ctx, tx, id)
	if err != nil {
		return jobdto.MutationResponse{}, err
	}
	if !containsStatus([]string{StatusFailed, StatusPartialSuccess, StatusCanceled}, current.Status) {
		return jobdto.MutationResponse{}, apperrors.BadRequest("当前状态不允许重试")
	}

	tag, err := tx.Exec(ctx, `
		UPDATE job_items
		SET status = CASE WHEN status = 'COMPLETED' THEN status ELSE 'WAITING_RETRY' END,
		    phase = NULL,
		    latest_error_code = NULL,
		    latest_error_message = NULL,
		    result_summary = NULL,
		    progress_percent = CASE WHEN status = 'COMPLETED' THEN progress_percent ELSE 0 END,
		    started_at = CASE WHEN status = 'COMPLETED' THEN started_at ELSE NULL END,
		    finished_at = CASE WHEN status = 'COMPLETED' THEN finished_at ELSE NULL END,
		    canceled_at = NULL,
		    updated_at = $2
		WHERE job_id = $1
	`, id, now)
	if err != nil {
		return jobdto.MutationResponse{}, err
	}
	if tag.RowsAffected() == 0 {
		return jobdto.MutationResponse{}, apperrors.BadRequest("当前作业没有可重试的子项")
	}

	_, err = tx.Exec(ctx, `
		UPDATE jobs
		SET status = $2,
		    latest_error_code = NULL,
		    latest_error_message = NULL,
		    outcome_summary = NULL,
		    canceled_at = NULL,
		    finished_at = NULL,
		    updated_at = $3
		WHERE id = $1
	`, id, StatusWaitingRetry, now)
	if err != nil {
		return jobdto.MutationResponse{}, err
	}
	if err := s.refreshJobAggregateTx(ctx, tx, id, now); err != nil {
		return jobdto.MutationResponse{}, err
	}

	event, err := s.insertEvent(ctx, tx, eventInsertInput{
		JobID:     id,
		EventType: EventJobRetried,
		Message:   "作业已进入重试队列",
		JobStatus: ptr(StatusWaitingRetry),
		CreatedAt: now,
	})
	if err != nil {
		return jobdto.MutationResponse{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return jobdto.MutationResponse{}, err
	}

	s.publish(event)
	s.wake()
	job, err := s.loadJobRecord(ctx, id)
	if err != nil {
		return jobdto.MutationResponse{}, err
	}
	return jobdto.MutationResponse{Message: "作业已进入重试队列", Job: job}, nil
}

func (s *Service) UpdatePriority(ctx context.Context, id string, priority string) (jobdto.MutationResponse, error) {
	if priority != PriorityLow && priority != PriorityNormal && priority != PriorityHigh {
		return jobdto.MutationResponse{}, apperrors.BadRequest("优先级无效")
	}

	now := s.now().UTC()
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return jobdto.MutationResponse{}, err
	}
	defer tx.Rollback(ctx)

	current, err := s.loadJobRowTx(ctx, tx, id)
	if err != nil {
		return jobdto.MutationResponse{}, err
	}
	if current.Priority == priority {
		return jobdto.MutationResponse{Message: "优先级未变化", Job: mapJobRow(current)}, nil
	}

	_, err = tx.Exec(ctx, `
		UPDATE jobs
		SET priority = $2,
		    updated_at = $3
		WHERE id = $1
	`, id, priority, now)
	if err != nil {
		return jobdto.MutationResponse{}, err
	}

	event, err := s.insertEvent(ctx, tx, eventInsertInput{
		JobID:     id,
		EventType: EventJobPriorityChanged,
		Message:   "作业优先级已更新",
		JobStatus: ptr(current.Status),
		Payload: map[string]any{
			"priority": priority,
		},
		CreatedAt: now,
	})
	if err != nil {
		return jobdto.MutationResponse{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return jobdto.MutationResponse{}, err
	}

	s.publish(event)
	if current.Status == StatusPending || current.Status == StatusQueued || current.Status == StatusWaitingRetry {
		s.wake()
	}
	job, err := s.loadJobRecord(ctx, id)
	if err != nil {
		return jobdto.MutationResponse{}, err
	}
	return jobdto.MutationResponse{Message: "优先级已更新", Job: job}, nil
}

func (s *Service) transitionJob(ctx context.Context, id string, allowed []string, nextStatus string, eventType string, message string, payload map[string]any) (jobdto.MutationResponse, error) {
	now := s.now().UTC()
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return jobdto.MutationResponse{}, err
	}
	defer tx.Rollback(ctx)

	current, err := s.loadJobRowTx(ctx, tx, id)
	if err != nil {
		return jobdto.MutationResponse{}, err
	}
	if !containsStatus(allowed, current.Status) {
		return jobdto.MutationResponse{}, apperrors.BadRequest("当前状态不允许执行该操作")
	}

	_, err = tx.Exec(ctx, `
		UPDATE jobs
		SET status = $2,
		    updated_at = $3
		WHERE id = $1
	`, id, nextStatus, now)
	if err != nil {
		return jobdto.MutationResponse{}, err
	}

	event, err := s.insertEvent(ctx, tx, eventInsertInput{
		JobID:     id,
		EventType: eventType,
		Message:   message,
		JobStatus: ptr(nextStatus),
		Payload:   payload,
		CreatedAt: now,
	})
	if err != nil {
		return jobdto.MutationResponse{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return jobdto.MutationResponse{}, err
	}

	s.publish(event)
	job, err := s.loadJobRecord(ctx, id)
	if err != nil {
		return jobdto.MutationResponse{}, err
	}
	return jobdto.MutationResponse{Message: message, Job: job}, nil
}

package jobs

import (
	"context"

	"github.com/jackc/pgx/v5"

	apperrors "mare/services/center/internal/errors"
	jobdto "mare/shared/contracts/dto/job"
)

func (s *Service) PauseJobItem(ctx context.Context, itemID string) (jobdto.ItemMutationResponse, error) {
	now := s.now().UTC()
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return jobdto.ItemMutationResponse{}, err
	}
	defer tx.Rollback(ctx)

	item, err := s.loadItemRowTx(ctx, tx, itemID)
	if err != nil {
		return jobdto.ItemMutationResponse{}, err
	}
	job, err := s.loadJobRowTx(ctx, tx, item.JobID)
	if err != nil {
		return jobdto.ItemMutationResponse{}, err
	}
	if !containsStatus([]string{ItemStatusPending, ItemStatusQueued, ItemStatusRunning, ItemStatusWaitingRetry}, item.Status) {
		return jobdto.ItemMutationResponse{}, apperrors.BadRequest("当前子任务状态不允许暂停")
	}
	if containsStatus([]string{StatusCompleted, StatusFailed, StatusCanceled}, job.Status) {
		return jobdto.ItemMutationResponse{}, apperrors.BadRequest("当前作业状态不允许暂停子任务")
	}

	var externalUpdate *externalTaskUpdate
	if item.Status == ItemStatusRunning {
		externalUpdate, err = s.pauseExternalTaskForItem(ctx, tx, itemID)
		if err != nil {
			return jobdto.ItemMutationResponse{}, err
		}
	}

	if _, err := tx.Exec(ctx, `
		UPDATE job_items
		SET status = $2,
		    phase = 'PAUSED',
		    updated_at = $3
		WHERE id = $1
	`, itemID, ItemStatusPaused, now); err != nil {
		return jobdto.ItemMutationResponse{}, err
	}

	if _, err := tx.Exec(ctx, `
		UPDATE jobs
		SET status = $2,
		    updated_at = $3
		WHERE id = $1
	`, item.JobID, StatusPaused, now); err != nil {
		return jobdto.ItemMutationResponse{}, err
	}
	if err := s.refreshJobAggregateTx(ctx, tx, item.JobID, now); err != nil {
		return jobdto.ItemMutationResponse{}, err
	}

	jobEvent, err := s.insertEvent(ctx, tx, eventInsertInput{
		JobID:     item.JobID,
		EventType: EventJobPaused,
		Message:   "作业已暂停",
		JobStatus: ptr(StatusPaused),
		CreatedAt: now,
	})
	if err != nil {
		return jobdto.ItemMutationResponse{}, err
	}
	itemEvent, err := s.insertEvent(ctx, tx, eventInsertInput{
		JobID:      item.JobID,
		JobItemID:  &itemID,
		EventType:  EventItemPaused,
		Message:    "子任务已暂停",
		JobStatus:  ptr(StatusPaused),
		ItemStatus: ptr(ItemStatusPaused),
		CreatedAt:  now,
	})
	if err != nil {
		return jobdto.ItemMutationResponse{}, err
	}
	var externalEvent *jobdto.StreamEvent
	if externalUpdate != nil {
		event, err := s.insertExternalTaskUpdateEvent(ctx, tx, item.JobID, *externalUpdate, now)
		if err != nil {
			return jobdto.ItemMutationResponse{}, err
		}
		externalEvent = &event
	}

	if item.Status == ItemStatusRunning {
		if _, err := tx.Exec(ctx, `
			UPDATE job_attempts
			SET status = 'CANCELED',
			    finished_at = $2
			WHERE job_item_id = $1
			  AND status = 'RUNNING'
		`, itemID, now); err != nil {
			return jobdto.ItemMutationResponse{}, err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return jobdto.ItemMutationResponse{}, err
	}

	if item.Status == ItemStatusRunning {
		s.interruptRunningItem(itemID)
	}
	s.publish(jobEvent)
	s.publish(itemEvent)
	if externalEvent != nil {
		s.publish(*externalEvent)
	}
	return s.loadItemMutationResponse(ctx, item.JobID, itemID, "子任务已暂停")
}

func (s *Service) ResumeJobItem(ctx context.Context, itemID string) (jobdto.ItemMutationResponse, error) {
	now := s.now().UTC()
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return jobdto.ItemMutationResponse{}, err
	}
	defer tx.Rollback(ctx)

	item, err := s.loadItemRowTx(ctx, tx, itemID)
	if err != nil {
		return jobdto.ItemMutationResponse{}, err
	}
	job, err := s.loadJobRowTx(ctx, tx, item.JobID)
	if err != nil {
		return jobdto.ItemMutationResponse{}, err
	}
	if item.Status != ItemStatusPaused {
		return jobdto.ItemMutationResponse{}, apperrors.BadRequest("当前子任务状态不允许继续")
	}
	if containsStatus([]string{StatusCompleted, StatusFailed, StatusCanceled}, job.Status) {
		return jobdto.ItemMutationResponse{}, apperrors.BadRequest("当前作业状态不允许继续子任务")
	}

	externalUpdate, err := s.resumeExternalTaskForItem(ctx, tx, itemID)
	if err != nil {
		return jobdto.ItemMutationResponse{}, err
	}

	if _, err := tx.Exec(ctx, `
		UPDATE job_items
		SET status = $2,
		    phase = NULL,
		    canceled_at = NULL,
		    finished_at = NULL,
		    updated_at = $3
		WHERE id = $1
	`, itemID, ItemStatusQueued, now); err != nil {
		return jobdto.ItemMutationResponse{}, err
	}

	if _, err := tx.Exec(ctx, `
		UPDATE jobs
		SET status = $2,
		    updated_at = $3
		WHERE id = $1
	`, item.JobID, StatusQueued, now); err != nil {
		return jobdto.ItemMutationResponse{}, err
	}
	if err := s.refreshJobAggregateTx(ctx, tx, item.JobID, now); err != nil {
		return jobdto.ItemMutationResponse{}, err
	}

	jobEvent, err := s.insertEvent(ctx, tx, eventInsertInput{
		JobID:     item.JobID,
		EventType: EventJobResumed,
		Message:   "作业已恢复排队",
		JobStatus: ptr(StatusQueued),
		CreatedAt: now,
	})
	if err != nil {
		return jobdto.ItemMutationResponse{}, err
	}
	itemEvent, err := s.insertEvent(ctx, tx, eventInsertInput{
		JobID:      item.JobID,
		JobItemID:  &itemID,
		EventType:  EventItemResumed,
		Message:    "子任务已恢复",
		JobStatus:  ptr(StatusQueued),
		ItemStatus: ptr(ItemStatusQueued),
		CreatedAt:  now,
	})
	if err != nil {
		return jobdto.ItemMutationResponse{}, err
	}
	var externalEvent *jobdto.StreamEvent
	if externalUpdate != nil {
		event, err := s.insertExternalTaskUpdateEvent(ctx, tx, item.JobID, *externalUpdate, now)
		if err != nil {
			return jobdto.ItemMutationResponse{}, err
		}
		externalEvent = &event
	}

	if err := tx.Commit(ctx); err != nil {
		return jobdto.ItemMutationResponse{}, err
	}

	s.publish(jobEvent)
	s.publish(itemEvent)
	if externalEvent != nil {
		s.publish(*externalEvent)
	}
	s.wake()
	return s.loadItemMutationResponse(ctx, item.JobID, itemID, "子任务已恢复")
}

func (s *Service) CancelJobItem(ctx context.Context, itemID string) (jobdto.ItemMutationResponse, error) {
	now := s.now().UTC()
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return jobdto.ItemMutationResponse{}, err
	}
	defer tx.Rollback(ctx)

	item, err := s.loadItemRowTx(ctx, tx, itemID)
	if err != nil {
		return jobdto.ItemMutationResponse{}, err
	}
	job, err := s.loadJobRowTx(ctx, tx, item.JobID)
	if err != nil {
		return jobdto.ItemMutationResponse{}, err
	}
	if !containsStatus([]string{ItemStatusPending, ItemStatusQueued, ItemStatusRunning, ItemStatusWaitingRetry, ItemStatusPaused}, item.Status) {
		return jobdto.ItemMutationResponse{}, apperrors.BadRequest("当前子任务状态不允许取消")
	}
	if containsStatus([]string{StatusCompleted, StatusFailed, StatusCanceled}, job.Status) {
		return jobdto.ItemMutationResponse{}, apperrors.BadRequest("当前作业状态不允许取消子任务")
	}

	var externalUpdate *externalTaskUpdate
	if item.Status != ItemStatusCompleted && item.Status != ItemStatusCanceled {
		externalUpdate, err = s.cancelExternalTaskForItem(ctx, tx, itemID)
		if err != nil {
			return jobdto.ItemMutationResponse{}, err
		}
	}

	if _, err := tx.Exec(ctx, `
		UPDATE job_items
		SET status = $2,
		    phase = 'CANCELED',
		    progress_percent = CASE WHEN progress_percent >= 100 THEN progress_percent ELSE 100 END,
		    canceled_at = COALESCE(canceled_at, $3),
		    finished_at = COALESCE(finished_at, $3),
		    updated_at = $3
		WHERE id = $1
	`, itemID, ItemStatusCanceled, now); err != nil {
		return jobdto.ItemMutationResponse{}, err
	}
	if item.Status == ItemStatusRunning {
		if _, err := tx.Exec(ctx, `
			UPDATE job_attempts
			SET status = 'CANCELED',
			    finished_at = $2
			WHERE job_item_id = $1
			  AND status = 'RUNNING'
		`, itemID, now); err != nil {
			return jobdto.ItemMutationResponse{}, err
		}
	}
	if err := s.refreshJobAggregateTx(ctx, tx, item.JobID, now); err != nil {
		return jobdto.ItemMutationResponse{}, err
	}

	itemEvent, err := s.insertEvent(ctx, tx, eventInsertInput{
		JobID:      item.JobID,
		JobItemID:  &itemID,
		EventType:  EventItemCanceled,
		Message:    "子任务已取消",
		JobStatus:  ptr(job.Status),
		ItemStatus: ptr(ItemStatusCanceled),
		CreatedAt:  now,
	})
	if err != nil {
		return jobdto.ItemMutationResponse{}, err
	}
	var externalEvent *jobdto.StreamEvent
	if externalUpdate != nil {
		event, err := s.insertExternalTaskUpdateEvent(ctx, tx, item.JobID, *externalUpdate, now)
		if err != nil {
			return jobdto.ItemMutationResponse{}, err
		}
		externalEvent = &event
	}

	if err := tx.Commit(ctx); err != nil {
		return jobdto.ItemMutationResponse{}, err
	}

	if item.Status == ItemStatusRunning {
		s.interruptRunningItem(itemID)
	}
	s.publish(itemEvent)
	if externalEvent != nil {
		s.publish(*externalEvent)
	}
	if job.Status == StatusPending || job.Status == StatusQueued || job.Status == StatusWaitingRetry {
		s.wake()
	}
	return s.loadItemMutationResponse(ctx, item.JobID, itemID, "子任务已取消")
}

func (s *Service) loadItemMutationResponse(ctx context.Context, jobID string, itemID string, message string) (jobdto.ItemMutationResponse, error) {
	job, err := s.loadJobRecord(ctx, jobID)
	if err != nil {
		return jobdto.ItemMutationResponse{}, err
	}
	item, err := s.loadJobItemRecord(ctx, itemID)
	if err != nil {
		return jobdto.ItemMutationResponse{}, err
	}
	return jobdto.ItemMutationResponse{
		Message: message,
		Job:     job,
		Item:    item,
	}, nil
}

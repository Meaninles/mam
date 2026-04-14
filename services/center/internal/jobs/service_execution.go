package jobs

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

func (s *Service) claimNextJob(ctx context.Context) (*jobRow, error) {
	now := s.now().UTC()
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	var id string
	err = tx.QueryRow(ctx, `
		SELECT id
		FROM jobs
		WHERE status IN ('PENDING', 'QUEUED')
		   OR (
			status = 'WAITING_RETRY'
			AND EXISTS (
				SELECT 1
				FROM job_items ji
				WHERE ji.job_id = jobs.id
				  AND ji.status = 'WAITING_RETRY'
				  AND (ji.next_retry_at IS NULL OR ji.next_retry_at <= $1)
			)
		   )
		ORDER BY
			CASE priority WHEN 'HIGH' THEN 0 WHEN 'NORMAL' THEN 1 ELSE 2 END,
			created_at ASC
		FOR UPDATE SKIP LOCKED
		LIMIT 1
	`, now).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	_, err = tx.Exec(ctx, `
		UPDATE jobs
		SET status = $2,
		    started_at = COALESCE(started_at, $3),
		    updated_at = $3
		WHERE id = $1
	`, id, StatusRunning, now)
	if err != nil {
		return nil, err
	}

	event, err := s.insertEvent(ctx, tx, eventInsertInput{
		JobID:     id,
		EventType: EventJobStarted,
		Message:   "作业开始执行",
		JobStatus: ptr(StatusRunning),
		CreatedAt: now,
	})
	if err != nil {
		return nil, err
	}

	row, err := s.loadJobRowTx(ctx, tx, id)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	s.publish(event)
	return &row, nil
}

func (s *Service) executeJob(ctx context.Context, job jobRow) {
	executor, ok := s.executors[job.JobIntent]
	if !ok {
		_ = s.failJobWithoutExecutor(ctx, job)
		return
	}

	_ = s.createJobAttempt(ctx, job.ID, nil, "RUNNING", nil, nil)

	items, err := s.loadItemRows(ctx, job.ID)
	if err != nil {
		_ = s.failJobWithError(ctx, job.ID, "load_items_failed", err.Error())
		return
	}
	jobLinks, err := s.loadJobLinks(ctx, job.ID)
	if err != nil {
		_ = s.failJobWithError(ctx, job.ID, "load_links_failed", err.Error())
		return
	}

	for _, item := range items {
		currentTime := s.now().UTC()
		latest, err := s.loadJobRecord(ctx, job.ID)
		if err != nil {
			_ = s.failJobWithError(ctx, job.ID, "reload_job_failed", err.Error())
			return
		}
		if latest.Status == StatusPaused || latest.Status == StatusCanceled {
			if latest.Status == StatusCanceled {
				_ = s.cancelPendingItems(ctx, job.ID)
			}
			return
		}
		if item.Status != ItemStatusPending && item.Status != ItemStatusQueued && item.Status != ItemStatusWaitingRetry {
			continue
		}
		if item.Status == ItemStatusWaitingRetry && item.NextRetryAt != nil && item.NextRetryAt.After(currentTime) {
			continue
		}

		jobAttemptID, err := s.startItem(ctx, job.ID, item.ID)
		if err != nil {
			_ = s.failJobWithError(ctx, job.ID, "start_item_failed", err.Error())
			return
		}

		jobSnapshot, err := s.loadJobRecord(ctx, job.ID)
		if err != nil {
			_ = s.failJobWithError(ctx, job.ID, "load_job_snapshot_failed", err.Error())
			return
		}
		itemSnapshot, err := s.loadJobItemRecord(ctx, item.ID)
		if err != nil {
			_ = s.failJobWithError(ctx, job.ID, "load_item_snapshot_failed", err.Error())
			return
		}
		itemLinks, err := s.loadItemLinks(ctx, item.ID)
		if err != nil {
			_ = s.failJobWithError(ctx, job.ID, "load_item_links_failed", err.Error())
			return
		}

		itemCtx, itemCancel := context.WithCancel(ctx)
		s.registerRunningItem(item.ID, itemCancel)
		execErr := executor(itemCtx, ExecutionContext{
			Job:       jobSnapshot,
			Item:      itemSnapshot,
			JobLinks:  jobLinks,
			ItemLinks: itemLinks,
		})
		itemCancel()
		s.clearRunningItem(item.ID)
		if execErr != nil {
			if handled, handleErr := s.handleItemExecutionInterruption(ctx, job.ID, item.ID); handled {
				if handleErr != nil {
					_ = s.failJobWithError(ctx, job.ID, "resolve_item_interruption_failed", handleErr.Error())
					return
				}
				continue
			}
			_ = s.finishItemFailed(ctx, job.ID, item.ID, jobAttemptID, execErr)
			continue
		}
		_ = s.finishItemCompleted(ctx, job.ID, item.ID, jobAttemptID)
	}

	if err := s.finalizeJob(ctx, job.ID); err != nil {
		_ = s.failJobWithError(ctx, job.ID, "finalize_job_failed", err.Error())
	}
}

func (s *Service) startItem(ctx context.Context, jobID string, itemID string) (*string, error) {
	now := s.now().UTC()
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	var nextAttempt int
	if err := tx.QueryRow(ctx, `
		SELECT COALESCE(MAX(attempt_no), 0) + 1
		FROM job_attempts
		WHERE job_item_id = $1
	`, itemID).Scan(&nextAttempt); err != nil {
		return nil, err
	}
	attemptID := buildCode("job-attempt-id", now)
	_, err = tx.Exec(ctx, `
		INSERT INTO job_attempts (
			id, job_id, job_item_id, attempt_no, status, worker_type, started_at
		) VALUES ($1, $2, $3, $4, 'RUNNING', 'CENTER', $5)
	`, attemptID, jobID, itemID, nextAttempt, now)
	if err != nil {
		return nil, err
	}
	_, err = tx.Exec(ctx, `
		UPDATE job_items
		SET status = $2,
		    phase = 'EXECUTING',
		    progress_percent = CASE WHEN progress_percent <= 0 THEN 1 ELSE progress_percent END,
		    attempt_count = $3,
		    next_retry_at = NULL,
		    started_at = COALESCE(started_at, $4),
		    updated_at = $4
		WHERE id = $1
	`, itemID, ItemStatusRunning, nextAttempt, now)
	if err != nil {
		return nil, err
	}
	if err := s.refreshJobAggregateTx(ctx, tx, jobID, now); err != nil {
		return nil, err
	}
	event, err := s.insertEvent(ctx, tx, eventInsertInput{
		JobID:      jobID,
		JobItemID:  &itemID,
		AttemptID:  &attemptID,
		EventType:  EventItemStarted,
		Message:    "作业子项开始执行",
		JobStatus:  ptr(StatusRunning),
		ItemStatus: ptr(ItemStatusRunning),
		CreatedAt:  now,
	})
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	s.publish(event)
	return &attemptID, nil
}

func (s *Service) handleItemExecutionInterruption(ctx context.Context, jobID string, itemID string) (bool, error) {
	item, err := s.loadJobItemRecord(ctx, itemID)
	if err != nil {
		return false, err
	}
	switch item.Status {
	case ItemStatusPaused:
		return true, nil
	case ItemStatusCanceled:
		if err := s.refreshJobAggregate(ctx, jobID, s.now().UTC()); err != nil {
			return true, err
		}
		return true, nil
	default:
		return false, nil
	}
}

func (s *Service) finishItemCompleted(ctx context.Context, jobID string, itemID string, attemptID *string) error {
	now := s.now().UTC()
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	_, err = tx.Exec(ctx, `
		UPDATE job_items
		SET status = $2,
		    phase = 'COMPLETED',
		    progress_percent = 100,
		    result_summary = '执行成功',
		    finished_at = $3,
		    updated_at = $3
		WHERE id = $1
	`, itemID, ItemStatusCompleted, now)
	if err != nil {
		return err
	}
	if attemptID != nil {
		_, err = tx.Exec(ctx, `
			UPDATE job_attempts
			SET status = 'SUCCEEDED',
			    finished_at = $2
			WHERE id = $1
		`, *attemptID, now)
		if err != nil {
			return err
		}
	}
	if err := s.refreshJobAggregateTx(ctx, tx, jobID, now); err != nil {
		return err
	}
	event, err := s.insertEvent(ctx, tx, eventInsertInput{
		JobID:      jobID,
		JobItemID:  &itemID,
		AttemptID:  attemptID,
		EventType:  EventItemCompleted,
		Message:    "作业子项执行成功",
		JobStatus:  ptr(StatusRunning),
		ItemStatus: ptr(ItemStatusCompleted),
		CreatedAt:  now,
	})
	if err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}
	s.publish(event)
	s.syncJobIssues(ctx, jobID)
	s.syncJobNotifications(ctx, jobID)
	return nil
}

func (s *Service) finishItemFailed(ctx context.Context, jobID string, itemID string, attemptID *string, execErr error) error {
	now := s.now().UTC()
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	message := strings.TrimSpace(execErr.Error())
	if message == "" {
		message = "执行失败"
	}
	job, err := s.loadJobRowTx(ctx, tx, jobID)
	if err != nil {
		return err
	}
	item, err := s.loadItemRowTx(ctx, tx, itemID)
	if err != nil {
		return err
	}
	if s.shouldAutoRetry(job, item, message) {
		nextRetryAt := now.Add(s.computeRetryDelay(item.AttemptCount))
		_, err = tx.Exec(ctx, `
			UPDATE job_items
			SET status = $2,
			    phase = 'WAITING_RETRY',
			    progress_percent = 0,
			    latest_error_code = 'execution_failed',
			    latest_error_message = $3,
			    result_summary = '等待自动重试',
			    next_retry_at = $4,
			    finished_at = NULL,
			    updated_at = $5
			WHERE id = $1
		`, itemID, ItemStatusWaitingRetry, message, nextRetryAt, now)
		if err != nil {
			return err
		}
		if attemptID != nil {
			_, err = tx.Exec(ctx, `
				UPDATE job_attempts
				SET status = 'FAILED',
				    error_code = 'execution_failed',
				    error_message = $2,
				    finished_at = $3
				WHERE id = $1
			`, *attemptID, message, now)
			if err != nil {
				return err
			}
		}
		_, err = tx.Exec(ctx, `
			UPDATE jobs
			SET status = $2,
			    latest_error_code = 'execution_failed',
			    latest_error_message = $3,
			    outcome_summary = '等待自动重试',
			    finished_at = NULL,
			    updated_at = $4
			WHERE id = $1
		`, jobID, StatusWaitingRetry, message, now)
		if err != nil {
			return err
		}
		if err := s.refreshJobAggregateTx(ctx, tx, jobID, now); err != nil {
			return err
		}
		event, err := s.insertEvent(ctx, tx, eventInsertInput{
			JobID:      jobID,
			JobItemID:  &itemID,
			AttemptID:  attemptID,
			EventType:  EventItemFailed,
			Message:    "作业子项执行失败，等待自动重试",
			JobStatus:  ptr(StatusWaitingRetry),
			ItemStatus: ptr(ItemStatusWaitingRetry),
			Payload: map[string]any{
				"error":       message,
				"nextRetryAt": nextRetryAt.Format(time.RFC3339),
			},
			CreatedAt: now,
		})
		if err != nil {
			return err
		}
		if err := tx.Commit(ctx); err != nil {
			return err
		}
		s.publish(event)
		s.wake()
		return nil
	}
	_, err = tx.Exec(ctx, `
		UPDATE job_items
		SET status = $2,
		    phase = 'FAILED',
		    progress_percent = 100,
		    latest_error_code = 'execution_failed',
		    latest_error_message = $3,
		    result_summary = '执行失败',
		    next_retry_at = NULL,
		    finished_at = $4,
		    updated_at = $4
		WHERE id = $1
	`, itemID, ItemStatusFailed, message, now)
	if err != nil {
		return err
	}
	if attemptID != nil {
		_, err = tx.Exec(ctx, `
			UPDATE job_attempts
			SET status = 'FAILED',
			    error_code = 'execution_failed',
			    error_message = $2,
			    finished_at = $3
			WHERE id = $1
		`, *attemptID, message, now)
		if err != nil {
			return err
		}
	}
	if err := s.refreshJobAggregateTx(ctx, tx, jobID, now); err != nil {
		return err
	}
	event, err := s.insertEvent(ctx, tx, eventInsertInput{
		JobID:      jobID,
		JobItemID:  &itemID,
		AttemptID:  attemptID,
		EventType:  EventItemFailed,
		Message:    "作业子项执行失败",
		JobStatus:  ptr(StatusRunning),
		ItemStatus: ptr(ItemStatusFailed),
		Payload: map[string]any{
			"error": message,
		},
		CreatedAt: now,
	})
	if err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}
	s.publish(event)
	s.syncJobIssues(ctx, jobID)
	s.syncJobNotifications(ctx, jobID)
	return nil
}

func (s *Service) finalizeJob(ctx context.Context, jobID string) error {
	now := s.now().UTC()
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	job, err := s.loadJobRowTx(ctx, tx, jobID)
	if err != nil {
		return err
	}
	if job.Status == StatusPaused || job.Status == StatusCanceled {
		if job.Status == StatusCanceled {
			if _, err := tx.Exec(ctx, `
				UPDATE job_items
				SET status = CASE WHEN status = 'COMPLETED' THEN status ELSE 'CANCELED' END,
				    progress_percent = CASE WHEN status = 'COMPLETED' THEN progress_percent ELSE 100 END,
				    canceled_at = COALESCE(canceled_at, $2),
				    finished_at = COALESCE(finished_at, $2),
				    updated_at = $2
				WHERE job_id = $1
			`, jobID, now); err != nil {
				return err
			}
			if err := s.refreshJobAggregateTx(ctx, tx, jobID, now); err != nil {
				return err
			}
		}
		return tx.Commit(ctx)
	}

	var completed int
	var failed int
	var canceled int
	var waitingRetry int
	if err := tx.QueryRow(ctx, `
		SELECT
			COUNT(*) FILTER (WHERE status = 'COMPLETED'),
			COUNT(*) FILTER (WHERE status = 'FAILED'),
			COUNT(*) FILTER (WHERE status = 'CANCELED'),
			COUNT(*) FILTER (WHERE status = 'WAITING_RETRY')
		FROM job_items
		WHERE job_id = $1
	`, jobID).Scan(&completed, &failed, &canceled, &waitingRetry); err != nil {
		return err
	}

	status := StatusCompleted
	eventType := EventJobCompleted
	message := "作业执行完成"
	switch {
	case waitingRetry > 0:
		status = StatusWaitingRetry
		eventType = EventJobRetried
		message = "作业等待自动重试"
	case completed == 0 && failed > 0:
		status = StatusFailed
		eventType = EventJobFailed
		message = "作业执行失败"
	case completed > 0 && failed > 0:
		status = StatusPartialSuccess
		eventType = EventJobPartialSuccess
		message = "作业部分成功"
	case completed == 0 && canceled > 0:
		status = StatusCanceled
		eventType = EventJobCanceled
		message = "作业已取消"
	}

	_, err = tx.Exec(ctx, `
		UPDATE jobs
		SET status = $2,
		    finished_at = CASE WHEN $2 IN ('COMPLETED', 'FAILED', 'PARTIAL_SUCCESS', 'CANCELED') THEN $3::timestamptz ELSE NULL END,
		    canceled_at = CASE WHEN $2 = 'CANCELED' THEN COALESCE(canceled_at, $3::timestamptz) ELSE canceled_at END,
		    updated_at = $3::timestamptz
		WHERE id = $1
	`, jobID, status, now)
	if err != nil {
		return err
	}
	if err := s.refreshJobAggregateTx(ctx, tx, jobID, now); err != nil {
		return err
	}
	event, err := s.insertEvent(ctx, tx, eventInsertInput{
		JobID:     jobID,
		EventType: eventType,
		Message:   message,
		JobStatus: ptr(status),
		CreatedAt: now,
	})
	if err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}
	s.publish(event)
	if status != StatusWaitingRetry {
		s.syncJobIssues(ctx, jobID)
		s.syncJobNotifications(ctx, jobID)
	}
	return nil
}

func (s *Service) failJobWithoutExecutor(ctx context.Context, job jobRow) error {
	return s.failJobWithError(ctx, job.ID, "executor_missing", "当前作业未注册执行器")
}

func (s *Service) failJobWithError(ctx context.Context, jobID string, code string, message string) error {
	now := s.now().UTC()
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	_, err = tx.Exec(ctx, `
		UPDATE jobs
		SET status = 'FAILED',
		    latest_error_code = $2,
		    latest_error_message = $3,
		    finished_at = $4,
		    updated_at = $4
		WHERE id = $1
	`, jobID, code, message, now)
	if err != nil {
		return err
	}
	event, err := s.insertEvent(ctx, tx, eventInsertInput{
		JobID:     jobID,
		EventType: EventJobFailed,
		Message:   "作业执行失败",
		JobStatus: ptr(StatusFailed),
		Payload: map[string]any{
			"code":  code,
			"error": message,
		},
		CreatedAt: now,
	})
	if err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}
	s.publish(event)
	s.syncJobIssues(ctx, jobID)
	s.syncJobNotifications(ctx, jobID)
	return nil
}

func (s *Service) cancelPendingItems(ctx context.Context, jobID string) error {
	now := s.now().UTC()
	_, err := s.pool.Exec(ctx, `
		UPDATE job_items
		SET status = CASE WHEN status = 'COMPLETED' THEN status ELSE 'CANCELED' END,
		    progress_percent = CASE WHEN status = 'COMPLETED' THEN progress_percent ELSE 100 END,
		    canceled_at = COALESCE(canceled_at, $2),
		    finished_at = COALESCE(finished_at, $2),
		    updated_at = $2
		WHERE job_id = $1
		  AND status <> 'COMPLETED'
	`, jobID, now)
	if err != nil {
		return err
	}
	return s.refreshJobAggregate(ctx, jobID, now)
}

func (s *Service) recoverQueuedJobs(ctx context.Context) error {
	now := s.now().UTC()
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `
		UPDATE jobs
		SET status = CASE WHEN status = 'RUNNING' THEN 'QUEUED' ELSE status END,
		    updated_at = $1
		WHERE status IN ('RUNNING', 'QUEUED', 'PENDING', 'WAITING_RETRY')
	`, now); err != nil {
		return err
	}

	if _, err := tx.Exec(ctx, `
		UPDATE job_items
		SET status = 'QUEUED',
		    phase = NULL,
		    updated_at = $1
		WHERE status = 'RUNNING'
	`, now); err != nil {
		return err
	}

	return tx.Commit(ctx)
}

package jobs

import (
	"context"

	"github.com/jackc/pgx/v5"
)

func (s *Service) UpdateExternalTask(ctx context.Context, jobID string, itemID string, engine string, taskID string, status string, payload map[string]any, resumeToken *string) error {
	now := s.now().UTC()
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	payloadJSON, err := marshalNullableJSON(payload)
	if err != nil {
		return err
	}

	if _, err := tx.Exec(ctx, `
		UPDATE job_items
		SET external_task_engine = $2,
		    external_task_id = $3,
		    external_task_status = $4,
		    external_task_payload = COALESCE($5, '{}'::jsonb),
		    resume_token = $6,
		    updated_at = $7
		WHERE id = $1
	`, itemID, engine, taskID, status, payloadJSON, resumeToken, now); err != nil {
		return err
	}

	event, err := s.insertEvent(ctx, tx, eventInsertInput{
		JobID:     jobID,
		JobItemID: &itemID,
		EventType: EventItemProgress,
		Message:   "外部任务状态已更新",
		CreatedAt: now,
		Payload: map[string]any{
			"externalTaskEngine": engine,
			"externalTaskId":     taskID,
			"externalTaskStatus": status,
		},
	})
	if err != nil {
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return err
	}

	s.publish(event)
	return nil
}

func (s *Service) UpdateItemTransferProgress(ctx context.Context, jobID string, itemID string, status string, bytesDone int64, bytesTotal int64, speedBPS int64, message string) error {
	now := s.now().UTC()
	progress := 0.0
	if bytesTotal > 0 {
		progress = (float64(bytesDone) / float64(bytesTotal)) * 100
		if progress > 99.5 && status != ItemStatusCompleted {
			progress = 99.5
		}
	}

	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `
		UPDATE job_items
		SET bytes_done = $2,
		    bytes_total = CASE WHEN $3 > 0 THEN $3 ELSE bytes_total END,
		    speed_bps = CASE WHEN $4 > 0 THEN $4 ELSE NULL END,
		    progress_percent = CASE WHEN $5 > progress_percent THEN $5 ELSE progress_percent END,
		    external_task_status = $6,
		    updated_at = $7
		WHERE id = $1
	`, itemID, bytesDone, bytesTotal, speedBPS, progress, status, now); err != nil {
		return err
	}

	if _, err := tx.Exec(ctx, `
		UPDATE jobs
		SET speed_bps = CASE WHEN $2 > 0 THEN $2 ELSE NULL END,
		    eta_seconds = CASE WHEN $2 > 0 AND $3 > $4 THEN (($3 - $4) / $2)::integer ELSE NULL END,
		    updated_at = $5
		WHERE id = $1
	`, jobID, speedBPS, bytesTotal, bytesDone, now); err != nil {
		return err
	}

	if err := s.refreshJobAggregateTx(ctx, tx, jobID, now); err != nil {
		return err
	}

	event, err := s.insertEvent(ctx, tx, eventInsertInput{
		JobID:     jobID,
		JobItemID: &itemID,
		EventType: EventItemProgress,
		Message:   "作业子项进度已更新",
		CreatedAt: now,
		Payload: map[string]any{
			"bytesDone":  bytesDone,
			"bytesTotal": bytesTotal,
			"speedBps":   speedBPS,
			"message":    message,
		},
	})
	if err != nil {
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return err
	}

	s.publish(event)
	return nil
}

func (s *Service) LoadExternalTaskState(ctx context.Context, itemID string) (string, *string, *string, *string, *string, error) {
	var (
		status             string
		taskEngine         *string
		taskID             *string
		externalTaskStatus *string
		resumeToken        *string
	)
	err := s.pool.QueryRow(ctx, `
		SELECT status, external_task_engine, external_task_id, external_task_status, resume_token
		FROM job_items
		WHERE id = $1
	`, itemID).Scan(&status, &taskEngine, &taskID, &externalTaskStatus, &resumeToken)
	if err != nil {
		return "", nil, nil, nil, nil, err
	}
	return status, taskEngine, taskID, externalTaskStatus, resumeToken, nil
}

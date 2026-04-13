package jobs

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"

	jobdto "mare/shared/contracts/dto/job"
)

type externalTaskUpdate struct {
	ItemID  string
	Engine  string
	TaskID  string
	Status  string
	Message string
}

func (s *Service) pauseExternalTaskForItem(ctx context.Context, tx pgx.Tx, itemID string) (*externalTaskUpdate, error) {
	if s.externalTasks == nil {
		return nil, nil
	}

	_, engine, taskID, _, _, err := s.loadExternalTaskStateTx(ctx, tx, itemID)
	if err != nil || engine == nil || taskID == nil {
		return nil, err
	}

	status, err := s.externalTasks.PauseExternalTask(ctx, *engine, *taskID)
	if err != nil {
		return nil, err
	}

	return &externalTaskUpdate{
		ItemID:  itemID,
		Engine:  *engine,
		TaskID:  *taskID,
		Status:  status,
		Message: "外部任务已暂停",
	}, nil
}

func (s *Service) resumeExternalTaskForItem(ctx context.Context, tx pgx.Tx, itemID string) (*externalTaskUpdate, error) {
	if s.externalTasks == nil {
		return nil, nil
	}

	_, engine, taskID, _, _, err := s.loadExternalTaskStateTx(ctx, tx, itemID)
	if err != nil || engine == nil || taskID == nil {
		return nil, err
	}

	status, err := s.externalTasks.ResumeExternalTask(ctx, *engine, *taskID)
	if err != nil {
		return nil, err
	}

	return &externalTaskUpdate{
		ItemID:  itemID,
		Engine:  *engine,
		TaskID:  *taskID,
		Status:  status,
		Message: "外部任务已恢复",
	}, nil
}

func (s *Service) cancelExternalTaskForItem(ctx context.Context, tx pgx.Tx, itemID string) (*externalTaskUpdate, error) {
	if s.externalTasks == nil {
		return nil, nil
	}

	_, engine, taskID, _, _, err := s.loadExternalTaskStateTx(ctx, tx, itemID)
	if err != nil || engine == nil || taskID == nil {
		return nil, err
	}

	status, err := s.externalTasks.CancelExternalTask(ctx, *engine, *taskID)
	if err != nil {
		return nil, err
	}

	return &externalTaskUpdate{
		ItemID:  itemID,
		Engine:  *engine,
		TaskID:  *taskID,
		Status:  status,
		Message: "外部任务已取消",
	}, nil
}

func (s *Service) insertExternalTaskUpdateEvent(ctx context.Context, tx pgx.Tx, jobID string, update externalTaskUpdate, createdAt time.Time) (jobdto.StreamEvent, error) {
	return s.insertEvent(ctx, tx, eventInsertInput{
		JobID:     jobID,
		JobItemID: &update.ItemID,
		EventType: EventItemProgress,
		Message:   update.Message,
		CreatedAt: createdAt,
		Payload: map[string]any{
			"externalTaskEngine": update.Engine,
			"externalTaskId":     update.TaskID,
			"externalTaskStatus": update.Status,
		},
	})
}

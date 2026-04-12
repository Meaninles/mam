package assets

import (
	"context"
	"testing"

	"mare/services/center/internal/integration"
)

func TestShouldResumeCD2Upload(t *testing.T) {
	paused := "Pause"
	if !shouldResumeCD2Upload("QUEUED", &paused) {
		t.Fatalf("expected queued paused upload to resume")
	}

	active := "Transfer"
	if shouldResumeCD2Upload("QUEUED", &active) {
		t.Fatalf("did not expect active upload to resume")
	}

	if !shouldResumeCD2Upload("RUNNING", &paused) {
		t.Fatalf("expected running paused upload to resume")
	}
}

func TestShouldResumeAria2Download(t *testing.T) {
	paused := "paused"
	if !shouldResumeAria2Download("QUEUED", &paused) {
		t.Fatalf("expected queued paused download to resume")
	}

	active := "active"
	if shouldResumeAria2Download("QUEUED", &active) {
		t.Fatalf("did not expect active download to resume")
	}

	if !shouldResumeAria2Download("RUNNING", &paused) {
		t.Fatalf("expected running paused download to resume")
	}
}

type fakeDownloadEngine struct {
	pausedTaskIDs []string
	canceledTaskIDs []string
}

func (f *fakeDownloadEngine) Name() string { return "ARIA2" }
func (f *fakeDownloadEngine) Start(context.Context) {}
func (f *fakeDownloadEngine) RuntimeStatus() integration.RuntimeComponent { return integration.RuntimeComponent{} }
func (f *fakeDownloadEngine) Enqueue(context.Context, integration.DownloadRequest) (string, error) { return "", nil }
func (f *fakeDownloadEngine) Recover(context.Context, string, integration.DownloadRequest) (string, error) { return "", nil }
func (f *fakeDownloadEngine) Wait(context.Context, string, func(integration.TransferProgress)) error { return nil }
func (f *fakeDownloadEngine) Pause(_ context.Context, taskID string) error {
	f.pausedTaskIDs = append(f.pausedTaskIDs, taskID)
	return nil
}
func (f *fakeDownloadEngine) Resume(context.Context, string) error { return nil }
func (f *fakeDownloadEngine) Cancel(_ context.Context, taskID string) error {
	f.canceledTaskIDs = append(f.canceledTaskIDs, taskID)
	return nil
}

type fakeJobRuntime struct {
	status string
	updateCalls []struct {
		jobID string
		itemID string
		engine string
		taskID string
		status string
	}
}

func (f *fakeJobRuntime) UpdateExternalTask(_ context.Context, jobID string, itemID string, engine string, taskID string, status string, _ map[string]any, _ *string) error {
	f.updateCalls = append(f.updateCalls, struct {
		jobID string
		itemID string
		engine string
		taskID string
		status string
	}{jobID: jobID, itemID: itemID, engine: engine, taskID: taskID, status: status})
	return nil
}

func (f *fakeJobRuntime) UpdateItemTransferProgress(context.Context, string, string, string, int64, int64, int64, string) error {
	return nil
}

func (f *fakeJobRuntime) LoadExternalTaskState(context.Context, string) (string, *string, *string, *string, *string, error) {
	return f.status, nil, nil, nil, nil, nil
}

func TestHandleCanceledDownloadWritesPausedExternalStatus(t *testing.T) {
	engine := &fakeDownloadEngine{}
	runtime := &fakeJobRuntime{status: "PAUSED"}
	service := &Service{jobRuntime: runtime}

	service.handleCanceledDownload(context.Background(), "job-1", "item-1", engine, "gid-1")

	if len(engine.pausedTaskIDs) != 1 || engine.pausedTaskIDs[0] != "gid-1" {
		t.Fatalf("expected downloader pause to be invoked, got %+v", engine.pausedTaskIDs)
	}
	if len(runtime.updateCalls) != 1 {
		t.Fatalf("expected external task update call, got %+v", runtime.updateCalls)
	}
	if runtime.updateCalls[0].status != "paused" {
		t.Fatalf("expected paused external status, got %+v", runtime.updateCalls[0])
	}
}

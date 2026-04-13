package assets

import (
	"context"
	"fmt"
	"testing"
	"time"

	"mare/services/center/internal/integration"
)

func TestShouldResumeCD2Upload(t *testing.T) {
	paused := "Pause"
	if !shouldResumeCD2Upload("QUEUED", &paused) {
		t.Fatalf("expected queued paused upload to resume")
	}

	cancelled := "Cancelled"
	if shouldResumeCD2Upload("QUEUED", &cancelled) {
		t.Fatalf("did not expect cancelled upload to resume")
	}

	active := "Transfer"
	if !shouldResumeCD2Upload("QUEUED", &active) {
		t.Fatalf("expected active upload to be explicitly resumed after recovery")
	}

	if !shouldResumeCD2Upload("RUNNING", &paused) {
		t.Fatalf("expected running paused upload to resume")
	}

	preprocessing := "Preprocessing"
	if !shouldResumeCD2Upload("RUNNING", &preprocessing) {
		t.Fatalf("expected preprocessing upload to be explicitly resumed after recovery")
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

func TestIsRecoverableCD2InterruptionError(t *testing.T) {
	if !isRecoverableCD2InterruptionError(fmt.Errorf("Interrupted upload cancelled")) {
		t.Fatalf("expected interrupted upload cancelled to be recoverable")
	}
	if !isRecoverableCD2InterruptionError(fmt.Errorf(`GeneralFailure Failed to read first segment: GeneralFailure("Upload session not found")`)) {
		t.Fatalf("expected upload session not found to be recoverable")
	}
	if isRecoverableCD2InterruptionError(fmt.Errorf("permission denied")) {
		t.Fatalf("did not expect unrelated error to be recoverable")
	}
}

type fakeDownloadEngine struct {
	pausedTaskIDs   []string
	canceledTaskIDs []string
}

type fakeCloudDriver struct {
	pausedTaskIDs   []string
	canceledTaskIDs []string
}

func (f *fakeCloudDriver) Vendor() string { return "115" }
func (f *fakeCloudDriver) AuthenticateToken(context.Context, string) (integration.ProviderAuthResult, error) {
	return integration.ProviderAuthResult{}, nil
}
func (f *fakeCloudDriver) CreateQRCodeSession(context.Context, string) (integration.QRCodeSession, error) {
	return integration.QRCodeSession{}, nil
}
func (f *fakeCloudDriver) GetQRCodeSession(context.Context, string) (integration.QRCodeSession, error) {
	return integration.QRCodeSession{}, nil
}
func (f *fakeCloudDriver) ConsumeQRCodeSession(context.Context, string) (integration.ProviderAuthResult, error) {
	return integration.ProviderAuthResult{}, nil
}
func (f *fakeCloudDriver) EnsureRemoteRoot(context.Context, integration.CloudProviderPayload, string) error { return nil }
func (f *fakeCloudDriver) StartUpload(context.Context, integration.CloudProviderPayload, string, string, integration.UploadSource) (string, string, error) {
	return "", "", nil
}
func (f *fakeCloudDriver) AttachUpload(context.Context, string, string, integration.UploadSource) error { return nil }
func (f *fakeCloudDriver) WaitUpload(context.Context, string, string, func(integration.TransferProgress)) error {
	return nil
}
func (f *fakeCloudDriver) ResetUploadSession(context.Context) error { return nil }
func (f *fakeCloudDriver) PauseUpload(_ context.Context, externalTaskID string) error {
	f.pausedTaskIDs = append(f.pausedTaskIDs, externalTaskID)
	return nil
}
func (f *fakeCloudDriver) ResumeUpload(context.Context, string) error { return nil }
func (f *fakeCloudDriver) CancelUpload(_ context.Context, externalTaskID string) error {
	f.canceledTaskIDs = append(f.canceledTaskIDs, externalTaskID)
	return nil
}
func (f *fakeCloudDriver) ResolveDownloadSource(context.Context, integration.CloudProviderPayload, string, string) (integration.DownloadSource, error) {
	return integration.DownloadSource{}, nil
}
func (f *fakeCloudDriver) DeleteFile(context.Context, integration.CloudProviderPayload, string, string) error { return nil }

func (f *fakeDownloadEngine) Name() string          { return "ARIA2" }
func (f *fakeDownloadEngine) Start(context.Context) {}
func (f *fakeDownloadEngine) RuntimeStatus() integration.RuntimeComponent {
	return integration.RuntimeComponent{}
}
func (f *fakeDownloadEngine) Enqueue(context.Context, integration.DownloadRequest) (string, error) {
	return "", nil
}
func (f *fakeDownloadEngine) Recover(context.Context, string, integration.DownloadRequest) (string, error) {
	return "", nil
}
func (f *fakeDownloadEngine) Wait(context.Context, string, func(integration.TransferProgress)) error {
	return nil
}
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
	status        string
	transferCalls []struct {
		jobID      string
		itemID     string
		status     string
		bytesDone  int64
		bytesTotal int64
		speedBPS   int64
		message    string
	}
	updateCalls []struct {
		jobID  string
		itemID string
		engine string
		taskID string
		status string
	}
}

func (f *fakeJobRuntime) UpdateExternalTask(_ context.Context, jobID string, itemID string, engine string, taskID string, status string, _ map[string]any, _ *string) error {
	f.updateCalls = append(f.updateCalls, struct {
		jobID  string
		itemID string
		engine string
		taskID string
		status string
	}{jobID: jobID, itemID: itemID, engine: engine, taskID: taskID, status: status})
	return nil
}

func (f *fakeJobRuntime) UpdateItemTransferProgress(_ context.Context, jobID string, itemID string, status string, bytesDone int64, bytesTotal int64, speedBPS int64, message string) error {
	f.transferCalls = append(f.transferCalls, struct {
		jobID      string
		itemID     string
		status     string
		bytesDone  int64
		bytesTotal int64
		speedBPS   int64
		message    string
	}{
		jobID:      jobID,
		itemID:     itemID,
		status:     status,
		bytesDone:  bytesDone,
		bytesTotal: bytesTotal,
		speedBPS:   speedBPS,
		message:    message,
	})
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

func TestHandleCanceledCloudUploadReturnsHandledForPausedTask(t *testing.T) {
	driver := &fakeCloudDriver{}
	runtime := &fakeJobRuntime{status: "PAUSED"}
	service := &Service{jobRuntime: runtime}

	handled := service.handleCanceledCloudUpload(context.Background(), "job-1", "item-1", driver, "upload-1")

	if !handled {
		t.Fatalf("expected paused upload cancellation to be handled")
	}
	if len(driver.pausedTaskIDs) != 1 || driver.pausedTaskIDs[0] != "upload-1" {
		t.Fatalf("expected pause upload call, got %+v", driver.pausedTaskIDs)
	}
	if len(runtime.updateCalls) != 1 || runtime.updateCalls[0].status != "Pause" {
		t.Fatalf("expected external pause status update, got %+v", runtime.updateCalls)
	}
}

func TestHandleCanceledCloudUploadLeavesRunningTaskForRecreate(t *testing.T) {
	driver := &fakeCloudDriver{}
	runtime := &fakeJobRuntime{status: "RUNNING"}
	service := &Service{jobRuntime: runtime}

	handled := service.handleCanceledCloudUpload(context.Background(), "job-1", "item-1", driver, "upload-1")

	if handled {
		t.Fatalf("expected running upload interruption to remain unhandled for recreate")
	}
	if len(driver.pausedTaskIDs) != 0 || len(driver.canceledTaskIDs) != 0 {
		t.Fatalf("did not expect driver control calls, got pause=%+v cancel=%+v", driver.pausedTaskIDs, driver.canceledTaskIDs)
	}
	if len(runtime.updateCalls) != 0 {
		t.Fatalf("did not expect external task updates, got %+v", runtime.updateCalls)
	}
}

func TestChooseSourceReplicaForReplicationPrefersLocalThenNASOverCloud(t *testing.T) {
	targetMountID := "mount-target"
	replicas := []operationReplica{
		{ID: "replica-cloud", MountID: "mount-cloud", NodeType: "CLOUD", ReplicaState: "AVAILABLE"},
		{ID: "replica-nas", MountID: "mount-nas", NodeType: "NAS", ReplicaState: "AVAILABLE"},
		{ID: "replica-local", MountID: "mount-local", NodeType: "LOCAL", ReplicaState: "AVAILABLE"},
	}

	selected := chooseSourceReplicaForReplication(replicas, targetMountID)
	if selected == nil {
		t.Fatalf("expected a source replica to be selected")
	}
	if selected.ID != "replica-local" {
		t.Fatalf("expected local replica to be preferred, got %+v", selected)
	}

	replicas = []operationReplica{
		{ID: "replica-cloud", MountID: "mount-cloud", NodeType: "CLOUD", ReplicaState: "AVAILABLE"},
		{ID: "replica-nas", MountID: "mount-nas", NodeType: "NAS", ReplicaState: "AVAILABLE"},
	}
	selected = chooseSourceReplicaForReplication(replicas, targetMountID)
	if selected == nil {
		t.Fatalf("expected a source replica to be selected")
	}
	if selected.ID != "replica-nas" {
		t.Fatalf("expected NAS replica to be preferred over cloud replica, got %+v", selected)
	}
}

func TestOpenUploadSourceRejectsCloudReplica(t *testing.T) {
	service := &Service{}

	source, err := service.openUploadSource(context.Background(), operationReplica{
		NodeType:     "CLOUD",
		PhysicalPath: "/115open/account-a/demo/file.bin",
	})
	if err == nil {
		if source != nil {
			_ = source.Close()
		}
		t.Fatalf("expected cloud replica to be rejected as upload source")
	}
}

func TestProgressNotifierThrottlesFrequentPersistence(t *testing.T) {
	now := time.Date(2026, 4, 13, 10, 0, 0, 0, time.UTC)
	runtime := &fakeJobRuntime{}
	service := &Service{
		now:        func() time.Time { return now },
		jobRuntime: runtime,
	}

	notify := service.progressNotifier("job-1", "item-1")
	notify(integration.TransferProgress{BytesDone: 10, BytesTotal: 100, SpeedBPS: 5, Message: "active"})
	if len(runtime.transferCalls) != 1 {
		t.Fatalf("expected first progress update to persist, got %d", len(runtime.transferCalls))
	}

	now = now.Add(1 * time.Second)
	notify(integration.TransferProgress{BytesDone: 12, BytesTotal: 100, SpeedBPS: 5, Message: "active"})
	if len(runtime.transferCalls) != 1 {
		t.Fatalf("expected frequent small update to be throttled, got %d", len(runtime.transferCalls))
	}

	now = now.Add(1 * time.Second)
	notify(integration.TransferProgress{BytesDone: 20, BytesTotal: 100, SpeedBPS: 5, Message: "active"})
	if len(runtime.transferCalls) != 2 {
		t.Fatalf("expected >=5%% progress jump to persist, got %d", len(runtime.transferCalls))
	}

	now = now.Add(6 * time.Second)
	notify(integration.TransferProgress{BytesDone: 21, BytesTotal: 100, SpeedBPS: 5, Message: "active"})
	if len(runtime.transferCalls) != 3 {
		t.Fatalf("expected timed flush to persist, got %d", len(runtime.transferCalls))
	}
}

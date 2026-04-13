package integration

import (
	"context"
	"fmt"
	"testing"
	"time"

	"google.golang.org/grpc/metadata"
	cd2pb "mare/services/center/internal/integration/cd2/pb"
)

type stubUploadSource struct{}

type stubRemoteUploadChannelClient struct{}
type failingRemoteUploadChannelClient struct{}

func (stubUploadSource) Size() int64 { return 0 }
func (stubUploadSource) ReadChunk(context.Context, int64, int64) ([]byte, bool, error) {
	return nil, true, nil
}
func (stubUploadSource) Close() error { return nil }

func (stubRemoteUploadChannelClient) Header() (metadata.MD, error) { return metadata.MD{}, nil }
func (stubRemoteUploadChannelClient) Trailer() metadata.MD         { return metadata.MD{} }
func (stubRemoteUploadChannelClient) CloseSend() error             { return nil }
func (stubRemoteUploadChannelClient) Context() context.Context     { return context.Background() }
func (stubRemoteUploadChannelClient) SendMsg(any) error            { return nil }
func (stubRemoteUploadChannelClient) RecvMsg(any) error            { return nil }
func (stubRemoteUploadChannelClient) Recv() (*cd2pb.RemoteUploadChannelReply, error) {
	return nil, context.Canceled
}

func (failingRemoteUploadChannelClient) Header() (metadata.MD, error) { return metadata.MD{}, nil }
func (failingRemoteUploadChannelClient) Trailer() metadata.MD         { return metadata.MD{} }
func (failingRemoteUploadChannelClient) CloseSend() error             { return nil }
func (failingRemoteUploadChannelClient) Context() context.Context     { return context.Background() }
func (failingRemoteUploadChannelClient) SendMsg(any) error            { return nil }
func (failingRemoteUploadChannelClient) RecvMsg(any) error            { return nil }
func (failingRemoteUploadChannelClient) Recv() (*cd2pb.RemoteUploadChannelReply, error) {
	return nil, context.DeadlineExceeded
}

func TestResolveCD2UploadStatusFallsBackToStatusString(t *testing.T) {
	item := &cd2pb.UploadFileInfo{
		Status: "Transfer",
	}

	got := resolveCD2UploadStatus(item)

	if got != cd2pb.UploadFileInfo_Transfer {
		t.Fatalf("expected Transfer, got %s", got.String())
	}
}

func TestIsCD2UploadTerminal(t *testing.T) {
	terminal := []cd2pb.UploadFileInfo_Status{
		cd2pb.UploadFileInfo_Finish,
		cd2pb.UploadFileInfo_Skipped,
		cd2pb.UploadFileInfo_Cancelled,
		cd2pb.UploadFileInfo_Error,
		cd2pb.UploadFileInfo_FatalError,
	}
	for _, status := range terminal {
		if !isCD2UploadTerminal(status) {
			t.Fatalf("expected %s to be terminal", status.String())
		}
	}

	nonTerminal := []cd2pb.UploadFileInfo_Status{
		cd2pb.UploadFileInfo_WaitforPreprocessing,
		cd2pb.UploadFileInfo_Preprocessing,
		cd2pb.UploadFileInfo_Inqueue,
		cd2pb.UploadFileInfo_Transfer,
		cd2pb.UploadFileInfo_Pause,
	}
	for _, status := range nonTerminal {
		if isCD2UploadTerminal(status) {
			t.Fatalf("expected %s to be non-terminal", status.String())
		}
	}
}

func TestDescribeCD2UploadStatus(t *testing.T) {
	cases := []struct {
		name   string
		status cd2pb.UploadFileInfo_Status
		err    string
		want   string
	}{
		{name: "preprocessing", status: cd2pb.UploadFileInfo_Preprocessing, want: "文件预处理中"},
		{name: "transfer", status: cd2pb.UploadFileInfo_Transfer, want: "上传中"},
		{name: "paused", status: cd2pb.UploadFileInfo_Pause, want: "已暂停"},
		{name: "finish", status: cd2pb.UploadFileInfo_Finish, want: "已完成"},
		{name: "fatal with error", status: cd2pb.UploadFileInfo_FatalError, err: "GeneralFailure", want: "GeneralFailure"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := describeCD2UploadStatus(tc.status, tc.err)
			if got != tc.want {
				t.Fatalf("expected %q, got %q", tc.want, got)
			}
		})
	}
}

func TestBuildCD2UploadStatusErrorUsesDefaults(t *testing.T) {
	err := buildCD2UploadStatusError(cd2pb.UploadFileInfo_FatalError.String(), "")
	if err == nil || err.Error() != "CloudDrive2 上传失败" {
		t.Fatalf("expected default fatal error, got %v", err)
	}

	err = buildCD2UploadStatusError(cd2pb.UploadFileInfo_Skipped.String(), "")
	if err == nil || err.Error() != "CloudDrive2 上传被跳过" {
		t.Fatalf("expected default skipped error, got %v", err)
	}
}

func TestAttachUploadRegistersRecoveredSession(t *testing.T) {
	driver := &CD2115Driver{
		deviceID:     "test-device",
		qrSessions:   map[string]*qrSessionState{},
		uploads:      map[string]*cd2UploadSession{},
		channelCtx:   context.Background(),
		uploadStream: stubRemoteUploadChannelClient{},
		uploadClient: &cd2Client{},
	}

	if err := driver.AttachUpload(context.Background(), "upload-1", "/cloud/file.bin", stubUploadSource{}); err != nil {
		t.Fatalf("attach upload: %v", err)
	}

	session := driver.uploads["upload-1"]
	if session == nil {
		t.Fatalf("expected upload session to be registered")
	}
	if session.destPath != "/cloud/file.bin" {
		t.Fatalf("expected destination path to be tracked, got %q", session.destPath)
	}
	if session.source == nil {
		t.Fatalf("expected upload source to be attached")
	}
}

func TestRunUploadChannelDisconnectDoesNotFailExistingSessions(t *testing.T) {
	client := &cd2Client{}
	stream := failingRemoteUploadChannelClient{}
	driver := &CD2115Driver{
		deviceID:     "test-device",
		qrSessions:   map[string]*qrSessionState{},
		uploads:      map[string]*cd2UploadSession{},
		channelCtx:   context.Background(),
		uploadStream: stream,
		uploadClient: client,
	}
	session := &cd2UploadSession{
		id:        "upload-1",
		destPath:  "/cloud/file.bin",
		source:    stubUploadSource{},
		done:      make(chan struct{}),
		hashJobs:  map[string]context.CancelFunc{},
		createdAt: time.Now().UTC(),
	}
	driver.uploads[session.id] = session

	driver.runUploadChannel(context.Background(), client, stream)

	select {
	case <-session.done:
		t.Fatalf("expected session to remain open after channel disconnect")
	default:
	}
	if session.err != nil {
		t.Fatalf("expected session error to stay nil, got %v", session.err)
	}
	if driver.uploadStream != nil || driver.uploadClient != nil || driver.channelCtx != nil {
		t.Fatalf("expected upload channel to be reset after disconnect")
	}
}

func TestHandleUploadStatusChangedPauseClearsHashJobsAndReadState(t *testing.T) {
	canceled := 0
	now := time.Now().UTC()
	session := &cd2UploadSession{
		id:          "upload-1",
		source:      stubUploadSource{},
		done:        make(chan struct{}),
		createdAt:   now,
		firstReadAt: &now,
		hashJobs: map[string]context.CancelFunc{
			"md5": func() { canceled++ },
		},
	}
	driver := &CD2115Driver{
		qrSessions: map[string]*qrSessionState{},
		uploads: map[string]*cd2UploadSession{
			session.id: session,
		},
	}

	driver.handleUploadStatusChanged("upload-1", &cd2pb.RemoteUploadStatusChanged{
		Status: cd2pb.UploadFileInfo_Pause,
	})

	if canceled != 1 {
		t.Fatalf("expected existing hash jobs to be canceled, got %d", canceled)
	}
	if len(session.hashJobs) != 0 {
		t.Fatalf("expected hash job registry to be cleared, got %+v", session.hashJobs)
	}
	if session.firstReadAt != nil {
		t.Fatalf("expected firstReadAt to be reset after pause")
	}
}

func TestIsCD2UploadSessionNotFoundError(t *testing.T) {
	if !isCD2UploadSessionNotFoundError(fmt.Errorf("GeneralFailure Upload session not found")) {
		t.Fatalf("expected upload session not found to be detected")
	}
	if isCD2UploadSessionNotFoundError(context.DeadlineExceeded) {
		t.Fatalf("did not expect unrelated error to be detected")
	}
}

func TestFailUploadResetsChannelWhenSessionNotFound(t *testing.T) {
	channelCtx, cancel := context.WithCancel(context.Background())
	driver := &CD2115Driver{
		qrSessions:   map[string]*qrSessionState{},
		uploads:      map[string]*cd2UploadSession{},
		channelCtx:   channelCtx,
		cancelChan:   cancel,
		uploadClient: &cd2Client{},
		uploadStream: stubRemoteUploadChannelClient{},
	}
	session := &cd2UploadSession{
		id:        "upload-1",
		source:    stubUploadSource{},
		done:      make(chan struct{}),
		hashJobs:  map[string]context.CancelFunc{},
		createdAt: time.Now().UTC(),
	}
	driver.uploads[session.id] = session

	driver.failUpload(session.id, fmt.Errorf("GeneralFailure Upload session not found"))

	select {
	case <-session.done:
	default:
		t.Fatalf("expected failed session to be closed")
	}
	if driver.channelCtx != nil || driver.cancelChan != nil || driver.uploadClient != nil || driver.uploadStream != nil {
		t.Fatalf("expected upload channel to be reset after session expiration")
	}
	select {
	case <-channelCtx.Done():
	default:
		t.Fatalf("expected upload channel context to be canceled")
	}
}

func TestResetUploadChannelIfCurrentDoesNotClearNewerChannel(t *testing.T) {
	newCtx, newCancel := context.WithCancel(context.Background())
	driver := &CD2115Driver{
		qrSessions:   map[string]*qrSessionState{},
		uploads:      map[string]*cd2UploadSession{},
		channelCtx:   newCtx,
		cancelChan:   newCancel,
		uploadClient: &cd2Client{},
		uploadStream: stubRemoteUploadChannelClient{},
	}
	oldClient := &cd2Client{}
	oldStream := failingRemoteUploadChannelClient{}

	driver.resetUploadChannelIfCurrent(oldClient, oldStream)

	if driver.channelCtx != newCtx || driver.cancelChan == nil || driver.uploadClient == nil || driver.uploadStream == nil {
		t.Fatalf("expected newer upload channel to remain intact")
	}
	select {
	case <-newCtx.Done():
		t.Fatalf("did not expect newer upload channel to be canceled")
	default:
	}
}

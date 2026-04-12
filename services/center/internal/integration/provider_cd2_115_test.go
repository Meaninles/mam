package integration

import (
	"context"
	"testing"

	"google.golang.org/grpc/metadata"
	cd2pb "mare/services/center/internal/integration/cd2/pb"
)

type stubUploadSource struct{}

type stubRemoteUploadChannelClient struct{}

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

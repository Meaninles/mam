package integration

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
)

func TestAria2TaskRecordDestinationPathUsesFirstFile(t *testing.T) {
	task := aria2TaskRecord{
		Dir: `C:\downloads`,
		Files: []aria2FileRecord{
			{Path: `C:\downloads\movie.mp4`},
		},
	}

	if got := task.destinationPath(); got != `C:\downloads\movie.mp4` {
		t.Fatalf("expected file path, got %q", got)
	}
}

func TestAria2TaskRecordDestinationPathFallsBackToDir(t *testing.T) {
	task := aria2TaskRecord{Dir: `C:\downloads`}
	if got := filepath.Clean(task.destinationPath()); got != filepath.Clean(`C:\downloads`) {
		t.Fatalf("expected dir fallback, got %q", got)
	}
}

func TestShouldReuseAria2TaskStatus(t *testing.T) {
	cases := []struct {
		status string
		want   bool
	}{
		{status: "active", want: true},
		{status: "waiting", want: true},
		{status: "paused", want: true},
		{status: "complete", want: true},
		{status: "error", want: false},
		{status: "removed", want: false},
		{status: "", want: false},
	}

	for _, tc := range cases {
		if got := shouldReuseAria2TaskStatus(tc.status); got != tc.want {
			t.Fatalf("status %q expected %v, got %v", tc.status, tc.want, got)
		}
	}
}

func TestRecoverAria2TaskRefreshesURLAndResumesErrorTask(t *testing.T) {
	var (
		changeCalled  bool
		resumeCalled  bool
		enqueueCalled bool
	)

	gid, err := recoverAria2Task(context.Background(), "gid-1", DownloadRequest{URL: "https://new.example/file.bin"}, aria2TellStatusResponse{
		Status: "error",
		Files: []aria2FileRecord{
			{
				Path: `C:\downloads\file.bin`,
				URIs: []struct {
					URI string `json:"uri"`
				}{
					{URI: "https://old.example/file.bin"},
				},
			},
		},
	}, aria2RecoverOps{
		changeURI: func(ctx context.Context, taskID string, oldURIs []string, newURI string) error {
			changeCalled = true
			if taskID != "gid-1" || len(oldURIs) != 1 || oldURIs[0] != "https://old.example/file.bin" || newURI != "https://new.example/file.bin" {
				t.Fatalf("unexpected changeURI inputs: %s %#v %s", taskID, oldURIs, newURI)
			}
			return nil
		},
		resume: func(ctx context.Context, taskID string) error {
			resumeCalled = true
			if taskID != "gid-1" {
				t.Fatalf("unexpected resume task id: %s", taskID)
			}
			return nil
		},
		cancel: func(context.Context, string) error {
			t.Fatalf("did not expect cancel to be called")
			return nil
		},
		enqueue: func(context.Context, DownloadRequest) (string, error) {
			enqueueCalled = true
			return "new-gid", nil
		},
	})
	if err != nil {
		t.Fatalf("recover task: %v", err)
	}
	if gid != "gid-1" {
		t.Fatalf("expected original gid to be reused, got %s", gid)
	}
	if !changeCalled || !resumeCalled {
		t.Fatalf("expected changeURI and resume to be called")
	}
	if enqueueCalled {
		t.Fatalf("did not expect enqueue fallback")
	}
}

func TestRecoverAria2TaskFallsBackToEnqueueWhenRefreshFails(t *testing.T) {
	var (
		cancelCalled  bool
		enqueueCalled bool
	)

	gid, err := recoverAria2Task(context.Background(), "gid-1", DownloadRequest{URL: "https://new.example/file.bin"}, aria2TellStatusResponse{
		Status: "error",
		Files: []aria2FileRecord{
			{
				Path: `C:\downloads\file.bin`,
				URIs: []struct {
					URI string `json:"uri"`
				}{
					{URI: "https://old.example/file.bin"},
				},
			},
		},
	}, aria2RecoverOps{
		changeURI: func(context.Context, string, []string, string) error {
			return errors.New("change failed")
		},
		resume: func(context.Context, string) error {
			t.Fatalf("did not expect resume when change failed")
			return nil
		},
		cancel: func(ctx context.Context, taskID string) error {
			cancelCalled = true
			if taskID != "gid-1" {
				t.Fatalf("unexpected cancel task id: %s", taskID)
			}
			return nil
		},
		enqueue: func(ctx context.Context, request DownloadRequest) (string, error) {
			enqueueCalled = true
			if request.URL != "https://new.example/file.bin" {
				t.Fatalf("unexpected enqueue request: %+v", request)
			}
			return "new-gid", nil
		},
	})
	if err != nil {
		t.Fatalf("recover task: %v", err)
	}
	if gid != "new-gid" {
		t.Fatalf("expected new gid after enqueue fallback, got %s", gid)
	}
	if !cancelCalled || !enqueueCalled {
		t.Fatalf("expected cancel and enqueue fallback to be called")
	}
}

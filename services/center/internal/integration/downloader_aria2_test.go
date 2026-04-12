package integration

import (
	"path/filepath"
	"testing"
)

func TestAria2TaskRecordDestinationPathUsesFirstFile(t *testing.T) {
	task := aria2TaskRecord{
		Dir: `C:\downloads`,
		Files: []struct {
			Path string `json:"path"`
		}{
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

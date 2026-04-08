package runtime

import (
	"encoding/json"
	"testing"
	"time"
)

func TestBuildSummaryIncludesRequiredFields(t *testing.T) {
	startedAt := time.Date(2026, 4, 8, 12, 0, 0, 0, time.UTC)

	summary, err := BuildSummary(Options{
		AgentID:   "agent-dev-1",
		Version:   "dev",
		Hostname:  "工作站-A",
		Platform:  "windows/amd64",
		Mode:      "attached",
		ProcessID: 1024,
		StartedAt: startedAt,
	})
	if err != nil {
		t.Fatalf("build summary: %v", err)
	}

	raw, err := json.Marshal(summary)
	if err != nil {
		t.Fatalf("marshal summary: %v", err)
	}

	expected := `{"agentId":"agent-dev-1","version":"dev","hostname":"工作站-A","platform":"windows/amd64","mode":"attached","processId":1024,"startedAt":"2026-04-08T12:00:00Z"}`
	if string(raw) != expected {
		t.Fatalf("unexpected json: %s", string(raw))
	}
}

func TestBuildSummaryRejectsMissingIdentity(t *testing.T) {
	_, err := BuildSummary(Options{
		Version:   "dev",
		Hostname:  "工作站-A",
		Platform:  "windows/amd64",
		Mode:      "attached",
		ProcessID: 1024,
		StartedAt: time.Now().UTC(),
	})
	if err == nil {
		t.Fatal("expected missing agent id error")
	}
}

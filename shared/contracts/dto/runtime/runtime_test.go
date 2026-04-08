package runtime

import (
	"encoding/json"
	"testing"
)

func TestRuntimeStatusResponseJSONShape(t *testing.T) {
	t.Parallel()

	payload := RuntimeStatusResponse{
		Status: "ready",
		Service: ServiceRuntimeStatus{
			Name:      "mare-center",
			Version:   "dev",
			Status:    "up",
			StartedAt: "2026-04-08T12:00:00Z",
		},
		Database: ComponentStatus{
			Status:  "up",
			Message: "数据库连接正常",
		},
		Migration: MigrationStatus{
			Status:         "ready",
			CurrentVersion: 1,
			LatestVersion:  1,
		},
		Agents: []AgentRuntimeStatus{
			{
				AgentID:         "agent-dev-1",
				Version:         "dev",
				Hostname:        "workstation",
				Platform:        "windows/amd64",
				Mode:            "attached",
				ProcessID:       1024,
				Status:          "online",
				RegisteredAt:    "2026-04-08T12:00:00Z",
				LastHeartbeatAt: "2026-04-08T12:00:10Z",
			},
		},
		Timestamp: "2026-04-08T12:00:10Z",
	}

	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal runtime status response: %v", err)
	}

	expected := `{"status":"ready","service":{"name":"mare-center","version":"dev","status":"up","startedAt":"2026-04-08T12:00:00Z"},"database":{"status":"up","message":"数据库连接正常"},"migration":{"status":"ready","currentVersion":1,"latestVersion":1},"agents":[{"agentId":"agent-dev-1","version":"dev","hostname":"workstation","platform":"windows/amd64","mode":"attached","processId":1024,"status":"online","registeredAt":"2026-04-08T12:00:00Z","lastHeartbeatAt":"2026-04-08T12:00:10Z"}],"timestamp":"2026-04-08T12:00:10Z"}`
	if string(raw) != expected {
		t.Fatalf("unexpected json: %s", string(raw))
	}
}

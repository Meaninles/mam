package agent

import (
	"encoding/json"
	"testing"
)

func TestRegisterRequestJSONShape(t *testing.T) {
	t.Parallel()

	payload := RegisterRequest{
		AgentID:         "agent-dev-1",
		Version:         "dev",
		Hostname:        "workstation",
		Platform:        "windows/amd64",
		Mode:            "attached",
		ProcessID:       1024,
		CallbackBaseURL: "http://127.0.0.1:61337",
		Capabilities: []string{
			"localfs",
		},
	}

	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal register request: %v", err)
	}

	expected := `{"agentId":"agent-dev-1","version":"dev","hostname":"workstation","platform":"windows/amd64","mode":"attached","processId":1024,"callbackBaseUrl":"http://127.0.0.1:61337","capabilities":["localfs"]}`
	if string(raw) != expected {
		t.Fatalf("unexpected json: %s", string(raw))
	}
}

func TestHeartbeatRequestSupportsUnicodeHostname(t *testing.T) {
	t.Parallel()

	payload := HeartbeatRequest{
		AgentID:         "agent-dev-1",
		Version:         "dev",
		Hostname:        "工作站-A",
		Platform:        "windows/amd64",
		Mode:            "attached",
		ProcessID:       2048,
		CallbackBaseURL: "http://127.0.0.1:61337",
		Capabilities: []string{
			"localfs",
			"storage-report",
		},
	}

	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal heartbeat request: %v", err)
	}

	expected := `{"agentId":"agent-dev-1","version":"dev","hostname":"工作站-A","platform":"windows/amd64","mode":"attached","processId":2048,"callbackBaseUrl":"http://127.0.0.1:61337","capabilities":["localfs","storage-report"]}`
	if string(raw) != expected {
		t.Fatalf("unexpected json: %s", string(raw))
	}
}

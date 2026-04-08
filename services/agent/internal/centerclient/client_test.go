package centerclient

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	agentdto "mare/shared/contracts/dto/agent"
)

func TestRegisterSendsExpectedPayload(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/agent/register" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}

		var payload agentdto.RegisterRequest
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode register payload: %v", err)
		}

		if payload.AgentID != "agent-dev-1" || payload.Hostname != "工作站-A" {
			t.Fatalf("unexpected payload: %+v", payload)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"data":{"agentId":"agent-dev-1"},"timestamp":"2026-04-08T12:00:00Z"}`))
	}))
	defer server.Close()

	client := New(server.URL, 2*time.Second)
	if err := client.Register(context.Background(), agentdto.RegisterRequest{
		AgentID:   "agent-dev-1",
		Version:   "dev",
		Hostname:  "工作站-A",
		Platform:  "windows/amd64",
		Mode:      "attached",
		ProcessID: 1024,
	}); err != nil {
		t.Fatalf("register: %v", err)
	}
}

func TestHeartbeatReturnsErrorForNotFound(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":{"code":"not_found","message":"执行器尚未注册"},"timestamp":"2026-04-08T12:00:00Z"}`))
	}))
	defer server.Close()

	client := New(server.URL, 2*time.Second)
	err := client.Heartbeat(context.Background(), agentdto.HeartbeatRequest{
		AgentID:   "agent-dev-1",
		Version:   "dev",
		Hostname:  "工作站-A",
		Platform:  "windows/amd64",
		Mode:      "attached",
		ProcessID: 1024,
	})
	if err == nil {
		t.Fatal("expected heartbeat error")
	}
}

func TestRegisterReturnsNetworkErrorWhenCenterUnavailable(t *testing.T) {
	t.Parallel()

	client := New("http://127.0.0.1:65530", 200*time.Millisecond)
	err := client.Register(context.Background(), agentdto.RegisterRequest{
		AgentID:   "agent-dev-1",
		Version:   "dev",
		Hostname:  "工作站-A",
		Platform:  "windows/amd64",
		Mode:      "attached",
		ProcessID: 1024,
	})
	if err == nil {
		t.Fatal("expected register network error")
	}
}

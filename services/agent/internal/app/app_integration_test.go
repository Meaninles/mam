package app

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"mare/services/agent/internal/centerclient"
)

func TestRunnerTalksToCenterEndpoints(t *testing.T) {
	var registerCalls atomic.Int32
	var heartbeatCalls atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/agent/register":
			registerCalls.Add(1)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data":      map[string]any{"agentId": "agent-dev-1"},
				"timestamp": "2026-04-08T12:00:00Z",
			})
		case "/agent/heartbeat":
			heartbeatCalls.Add(1)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data":      map[string]any{"agentId": "agent-dev-1"},
				"timestamp": "2026-04-08T12:00:01Z",
			})
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	client := centerclient.New(server.URL, 2*time.Second)
	runner := NewRunner(client, Options{
		AgentID:           "agent-dev-1",
		Version:           "dev",
		Hostname:          "工作站-A",
		Platform:          "windows/amd64",
		Mode:              "attached",
		ProcessID:         1024,
		HeartbeatInterval: 20 * time.Millisecond,
		RetryDelay:        10 * time.Millisecond,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Millisecond)
	defer cancel()

	if err := runner.Run(ctx); err != nil {
		t.Fatalf("run runner: %v", err)
	}

	if registerCalls.Load() < 1 {
		t.Fatalf("expected register call, got %d", registerCalls.Load())
	}
	if heartbeatCalls.Load() < 1 {
		t.Fatalf("expected heartbeat call, got %d", heartbeatCalls.Load())
	}
}

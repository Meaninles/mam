package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"mare/services/center/internal/agentregistry"
	"mare/services/center/internal/db"
	"mare/services/center/internal/runtime"
)

type fakeRuntimeService struct {
	readyPayload  runtime.ReadinessPayload
	statusPayload runtime.RuntimeStatusPayload
}

func (f fakeRuntimeService) Health() runtime.HealthPayload {
	return runtime.HealthPayload{
		Status:    "up",
		Service:   "mare-center",
		Version:   "dev",
		Timestamp: "2026-04-08T12:00:00Z",
	}
}

func (f fakeRuntimeService) Ready(context.Context) (runtime.ReadinessPayload, error) {
	return f.readyPayload, nil
}

func (f fakeRuntimeService) Status(context.Context) (runtime.RuntimeStatusPayload, error) {
	return f.statusPayload, nil
}

type fakeAgentService struct{}

func (fakeAgentService) Register(_ context.Context, registration agentregistry.Registration) (agentregistry.Agent, error) {
	return agentregistry.Agent{
		AgentID:   registration.AgentID,
		Version:   registration.Version,
		Hostname:  registration.Hostname,
		Platform:  registration.Platform,
		Mode:      registration.Mode,
		ProcessID: registration.ProcessID,
	}, nil
}

func (fakeAgentService) Heartbeat(_ context.Context, heartbeat agentregistry.Heartbeat) (agentregistry.Agent, error) {
	return agentregistry.Agent{
		AgentID:   heartbeat.AgentID,
		Version:   heartbeat.Version,
		Hostname:  heartbeat.Hostname,
		Platform:  heartbeat.Platform,
		Mode:      heartbeat.Mode,
		ProcessID: heartbeat.ProcessID,
	}, nil
}

func TestHealthzReturnsSuccessEnvelope(t *testing.T) {
	t.Parallel()

	router := NewRouter(Dependencies{
		Runtime: fakeRuntimeService{},
		Agents:  fakeAgentService{},
	})

	request := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", recorder.Code)
	}
}

func TestReadyzReturnsNotReadyState(t *testing.T) {
	t.Parallel()

	router := NewRouter(Dependencies{
		Runtime: fakeRuntimeService{
			readyPayload: runtime.ReadinessPayload{
				Status: "not_ready",
				Service: runtime.ComponentStatus{
					Status:  "up",
					Message: "中心服务已启动",
				},
				Database: runtime.ComponentStatus{
					Status:  "down",
					Message: "数据库连接失败",
				},
				Migration: db.MigrationState{
					Status:         "pending",
					CurrentVersion: 0,
					LatestVersion:  1,
				},
				Version:   "dev",
				Timestamp: "2026-04-08T12:00:00Z",
			},
		},
		Agents: fakeAgentService{},
	})

	request := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", recorder.Code)
	}
}

func TestRegisterRouteAcceptsValidPayload(t *testing.T) {
	t.Parallel()

	router := NewRouter(Dependencies{
		Runtime: fakeRuntimeService{},
		Agents:  fakeAgentService{},
	})

	body, err := json.Marshal(agentregistry.Registration{
		AgentID:   "agent-dev-1",
		Version:   "dev",
		Hostname:  "工作站-A",
		Platform:  "windows/amd64",
		Mode:      "attached",
		ProcessID: 1024,
	})
	if err != nil {
		t.Fatalf("marshal register payload: %v", err)
	}

	request := httptest.NewRequest(http.MethodPost, "/agent/register", bytes.NewReader(body))
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", recorder.Code)
	}
}

func TestHeartbeatRouteAcceptsValidPayload(t *testing.T) {
	t.Parallel()

	router := NewRouter(Dependencies{
		Runtime: fakeRuntimeService{},
		Agents:  fakeAgentService{},
	})

	body, err := json.Marshal(agentregistry.Heartbeat{
		AgentID:   "agent-dev-1",
		Version:   "dev",
		Hostname:  "工作站-A",
		Platform:  "windows/amd64",
		Mode:      "attached",
		ProcessID: 1024,
	})
	if err != nil {
		t.Fatalf("marshal heartbeat payload: %v", err)
	}

	request := httptest.NewRequest(http.MethodPost, "/agent/heartbeat", bytes.NewReader(body))
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", recorder.Code)
	}
}

func TestRuntimeStatusRouteReturnsAggregatedState(t *testing.T) {
	t.Parallel()

	router := NewRouter(Dependencies{
		Runtime: fakeRuntimeService{
			statusPayload: runtime.RuntimeStatusPayload{
				Status: "ready",
				Service: runtime.ServiceRuntimeStatus{
					Name:      "mare-center",
					Version:   "dev",
					Status:    "up",
					StartedAt: "2026-04-08T12:00:00Z",
				},
				Database: runtime.ComponentStatus{
					Status:  "up",
					Message: "数据库连接正常",
				},
				Migration: db.MigrationState{
					Status:         "ready",
					CurrentVersion: 1,
					LatestVersion:  1,
				},
				Agents: []runtime.AgentRuntimeStatus{
					{
						AgentID:         "agent-dev-1",
						Version:         "dev",
						Hostname:        "工作站-A",
						Platform:        "windows/amd64",
						Mode:            "attached",
						ProcessID:       1024,
						Status:          "online",
						RegisteredAt:    "2026-04-08T12:00:00Z",
						LastHeartbeatAt: "2026-04-08T12:00:10Z",
					},
				},
				Timestamp: "2026-04-08T12:00:10Z",
			},
		},
		Agents: fakeAgentService{},
	})

	request := httptest.NewRequest(http.MethodGet, "/api/runtime/status", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", recorder.Code)
	}
}

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
	storagedto "mare/shared/contracts/dto/storage"
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
type fakeLocalFolderService struct{}

func (fakeAgentService) Register(_ context.Context, registration agentregistry.Registration) (agentregistry.Agent, error) {
	return agentregistry.Agent{
		AgentID:   registration.AgentID,
		Version:   registration.Version,
		Hostname:  registration.Hostname,
		Platform:  registration.Platform,
		Mode:      registration.Mode,
		ProcessID: registration.ProcessID,
		Capabilities: registration.Capabilities,
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
		Capabilities: heartbeat.Capabilities,
	}, nil
}

func (fakeLocalFolderService) ListLocalFolders(context.Context) ([]storagedto.LocalFolderRecord, error) {
	return []storagedto.LocalFolderRecord{
		{
			ID:               "local-folder-1",
			Name:             "商业摄影原片库",
			LibraryID:        "photo",
			LibraryName:      "商业摄影资产库",
			FolderType:       "本地",
			Address:          "D:\\Mare\\Assets\\PhotoRaw",
			MountMode:        "可写",
			Enabled:          true,
			ScanStatus:       "最近扫描成功",
			ScanTone:         "success",
			LastScanAt:       "今天 09:12",
			HeartbeatPolicy:  "从不",
			NextHeartbeatAt:  "—",
			CapacitySummary:  "待检测",
			FreeSpaceSummary: "待检测",
			CapacityPercent:  0,
			RiskTags:         []string{},
			Badges:           []string{"本地", "可写"},
			AuthStatus:       "无需鉴权",
			AuthTone:         "info",
		},
	}, nil
}

func (fakeLocalFolderService) SaveLocalFolder(_ context.Context, request storagedto.SaveLocalFolderRequest) (storagedto.SaveLocalFolderResponse, error) {
	return storagedto.SaveLocalFolderResponse{
		Message: "挂载文件夹已保存",
		Record: storagedto.LocalFolderRecord{
			ID:   request.ID,
			Name: request.Name,
		},
	}, nil
}

func (fakeLocalFolderService) RunLocalFolderScan(context.Context, []string) (storagedto.RunLocalFolderScanResponse, error) {
	return storagedto.RunLocalFolderScanResponse{Message: "已为 1 个挂载文件夹创建扫描任务"}, nil
}

func (fakeLocalFolderService) RunLocalFolderConnectionTest(context.Context, []string) (storagedto.RunLocalFolderConnectionTestResponse, error) {
	return storagedto.RunLocalFolderConnectionTestResponse{
		Message: "连接测试已完成",
		Results: []storagedto.ConnectionTestResult{
			{
				ID:          "local-folder-1",
				Name:        "商业摄影原片库",
				OverallTone: "success",
				Summary:     "目录可访问。",
				TestedAt:    "刚刚",
			},
		},
	}, nil
}

func (fakeLocalFolderService) UpdateLocalFolderHeartbeat(context.Context, []string, string) (storagedto.UpdateHeartbeatResponse, error) {
	return storagedto.UpdateHeartbeatResponse{Message: "心跳策略已更新"}, nil
}

func (fakeLocalFolderService) LoadLocalFolderScanHistory(_ context.Context, id string) (storagedto.LocalFolderScanHistoryResponse, error) {
	return storagedto.LocalFolderScanHistoryResponse{
		ID: id,
		Items: []storagedto.ScanHistoryItem{
			{
				ID:         "history-1",
				StartedAt:  "2026-04-08 09:12",
				FinishedAt: "2026-04-08 09:13",
				Status:     "成功",
				Summary:    "完成扫描。",
				Trigger:    "手动扫描",
			},
		},
	}, nil
}

func (fakeLocalFolderService) DeleteLocalFolder(context.Context, string) (storagedto.DeleteLocalFolderResponse, error) {
	return storagedto.DeleteLocalFolderResponse{Message: "挂载文件夹已删除"}, nil
}

func TestHealthzReturnsSuccessEnvelope(t *testing.T) {
	t.Parallel()

	router := NewRouter(Dependencies{
		Runtime:      fakeRuntimeService{},
		Agents:       fakeAgentService{},
		LocalFolders: fakeLocalFolderService{},
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
		Agents:       fakeAgentService{},
		LocalFolders: fakeLocalFolderService{},
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
		Runtime:      fakeRuntimeService{},
		Agents:       fakeAgentService{},
		LocalFolders: fakeLocalFolderService{},
	})

	body, err := json.Marshal(agentregistry.Registration{
		AgentID:   "agent-dev-1",
		Version:   "dev",
		Hostname:  "工作站-A",
		Platform:  "windows/amd64",
		Mode:      "attached",
		ProcessID: 1024,
		Capabilities: []string{
			"localfs",
		},
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
		Runtime:      fakeRuntimeService{},
		Agents:       fakeAgentService{},
		LocalFolders: fakeLocalFolderService{},
	})

	body, err := json.Marshal(agentregistry.Heartbeat{
		AgentID:   "agent-dev-1",
		Version:   "dev",
		Hostname:  "工作站-A",
		Platform:  "windows/amd64",
		Mode:      "attached",
		ProcessID: 1024,
		Capabilities: []string{
			"localfs",
		},
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
		Agents:       fakeAgentService{},
		LocalFolders: fakeLocalFolderService{},
	})

	request := httptest.NewRequest(http.MethodGet, "/api/runtime/status", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", recorder.Code)
	}
}

func TestRuntimeStatusRouteIncludesCORSHeaders(t *testing.T) {
	t.Parallel()

	router := NewRouter(Dependencies{
		Runtime: fakeRuntimeService{
			statusPayload: runtime.RuntimeStatusPayload{
				Status: "ready",
			},
		},
		Agents:       fakeAgentService{},
		LocalFolders: fakeLocalFolderService{},
	})

	request := httptest.NewRequest(http.MethodGet, "/api/runtime/status", nil)
	request.Header.Set("Origin", "http://localhost:5173")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	if recorder.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Fatalf("expected wildcard CORS header, got %q", recorder.Header().Get("Access-Control-Allow-Origin"))
	}
}

func TestOptionsPreflightReturnsNoContent(t *testing.T) {
	t.Parallel()

	router := NewRouter(Dependencies{
		Runtime:      fakeRuntimeService{},
		Agents:       fakeAgentService{},
		LocalFolders: fakeLocalFolderService{},
	})

	request := httptest.NewRequest(http.MethodOptions, "/api/runtime/status", nil)
	request.Header.Set("Origin", "http://localhost:5173")
	request.Header.Set("Access-Control-Request-Method", "GET")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", recorder.Code)
	}

	if recorder.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Fatalf("expected wildcard CORS header, got %q", recorder.Header().Get("Access-Control-Allow-Origin"))
	}
}

func TestLocalFoldersRouteReturnsPersistedRecords(t *testing.T) {
	t.Parallel()

	router := NewRouter(Dependencies{
		Runtime:      fakeRuntimeService{},
		Agents:       fakeAgentService{},
		LocalFolders: fakeLocalFolderService{},
	})

	request := httptest.NewRequest(http.MethodGet, "/api/storage/local-folders", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", recorder.Code)
	}
}

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
	assetdto "mare/shared/contracts/dto/asset"
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
type fakeLocalNodeService struct{}
type fakeNASNodeService struct{}
type fakeAssetService struct{}

func (fakeAgentService) Register(_ context.Context, registration agentregistry.Registration) (agentregistry.Agent, error) {
	return agentregistry.Agent{
		AgentID:      registration.AgentID,
		Version:      registration.Version,
		Hostname:     registration.Hostname,
		Platform:     registration.Platform,
		Mode:         registration.Mode,
		ProcessID:    registration.ProcessID,
		Capabilities: registration.Capabilities,
	}, nil
}

func (fakeAgentService) Heartbeat(_ context.Context, heartbeat agentregistry.Heartbeat) (agentregistry.Agent, error) {
	return agentregistry.Agent{
		AgentID:      heartbeat.AgentID,
		Version:      heartbeat.Version,
		Hostname:     heartbeat.Hostname,
		Platform:     heartbeat.Platform,
		Mode:         heartbeat.Mode,
		ProcessID:    heartbeat.ProcessID,
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

func (fakeLocalNodeService) ListLocalNodes(context.Context) ([]storagedto.LocalNodeRecord, error) {
	return []storagedto.LocalNodeRecord{
		{
			ID:           "local-node-1",
			Name:         "本地素材根目录",
			RootPath:     `D:\Assets`,
			Enabled:      true,
			HealthStatus: "可用",
			HealthTone:   "success",
			LastCheckAt:  "今天 09:12",
			MountCount:   1,
		},
	}, nil
}

func (fakeLocalNodeService) SaveLocalNode(_ context.Context, request storagedto.SaveLocalNodeRequest) (storagedto.SaveLocalNodeResponse, error) {
	return storagedto.SaveLocalNodeResponse{
		Message: "本地文件夹已保存",
		Record: storagedto.LocalNodeRecord{
			ID:       request.ID,
			Name:     request.Name,
			RootPath: request.RootPath,
		},
	}, nil
}

func (fakeLocalNodeService) RunLocalNodeConnectionTest(context.Context, []string) (storagedto.RunLocalNodeConnectionTestResponse, error) {
	return storagedto.RunLocalNodeConnectionTestResponse{
		Message: "连接测试已完成",
		Results: []storagedto.ConnectionTestResult{
			{ID: "local-node-1", Name: "本地素材根目录", OverallTone: "success", Summary: "根目录可访问", TestedAt: "刚刚"},
		},
	}, nil
}

func (fakeLocalNodeService) DeleteLocalNode(context.Context, string) (storagedto.DeleteLocalNodeResponse, error) {
	return storagedto.DeleteLocalNodeResponse{Message: "本地文件夹已删除"}, nil
}

func (fakeNASNodeService) ListNasNodes(context.Context) ([]storagedto.NasRecord, error) {
	return []storagedto.NasRecord{
		{
			ID:           "nas-node-1",
			Name:         "影像 NAS 01",
			Address:      `\\192.168.10.20\media`,
			AccessMode:   "SMB",
			Username:     "mare-sync",
			PasswordHint: "已保存",
			LastTestAt:   "今天 10:20",
			Status:       "鉴权正常",
			Tone:         "success",
			MountCount:   2,
		},
	}, nil
}

func (fakeNASNodeService) SaveNasNode(_ context.Context, request storagedto.SaveNasNodeRequest) (storagedto.SaveNasNodeResponse, error) {
	return storagedto.SaveNasNodeResponse{
		Message: "NAS 已保存",
		Record: storagedto.NasRecord{
			ID:           request.ID,
			Name:         request.Name,
			Address:      request.Address,
			AccessMode:   "SMB",
			Username:     request.Username,
			PasswordHint: "已保存",
			Status:       "待检测",
			Tone:         "info",
		},
	}, nil
}

func (fakeNASNodeService) RunNasNodeConnectionTest(context.Context, []string) (storagedto.RunNasNodeConnectionTestResponse, error) {
	return storagedto.RunNasNodeConnectionTestResponse{
		Message: "连接测试已完成",
		Results: []storagedto.ConnectionTestResult{
			{ID: "nas-node-1", Name: "影像 NAS 01", OverallTone: "success", Summary: "SMB 鉴权正常", TestedAt: "刚刚"},
		},
	}, nil
}

func (fakeNASNodeService) DeleteNasNode(context.Context, string) (storagedto.DeleteNasNodeResponse, error) {
	return storagedto.DeleteNasNodeResponse{Message: "NAS 已删除"}, nil
}

func (fakeAssetService) ListLibraries(context.Context) ([]assetdto.LibraryRecord, error) {
	return []assetdto.LibraryRecord{
		{
			ID:            "photo",
			Name:          "商业摄影资产库",
			RootLabel:     "/",
			ItemCount:     "1",
			Health:        "100%",
			StoragePolicy: "本地",
			EndpointNames: []string{"商业摄影原片库"},
		},
	}, nil
}

func (fakeAssetService) CreateLibrary(_ context.Context, request assetdto.CreateLibraryRequest) (assetdto.CreateLibraryResponse, error) {
	return assetdto.CreateLibraryResponse{
		Message: "资产库已创建",
		Library: assetdto.LibraryRecord{
			ID:            "library-photo",
			Name:          request.Name,
			RootLabel:     "/",
			ItemCount:     "0",
			Health:        "100%",
			StoragePolicy: "未绑定端点",
			EndpointNames: []string{},
		},
	}, nil
}

func (fakeAssetService) CreateDirectory(_ context.Context, libraryID string, request assetdto.CreateDirectoryRequest) (assetdto.CreateDirectoryResponse, error) {
	return assetdto.CreateDirectoryResponse{
		Message: "目录已创建",
		Entry: assetdto.EntryRecord{
			ID:             "dir-new",
			LibraryID:      libraryID,
			ParentID:       request.ParentID,
			Type:           "folder",
			LifecycleState: "ACTIVE",
			Name:           request.Name,
			FileKind:       "文件夹",
			DisplayType:    "文件夹",
			ModifiedAt:     "2026-04-10 12:20",
			CreatedAt:      "2026-04-10 12:20",
			Size:           "0 项",
			Path:           "商业摄影资产库 / " + request.Name,
			SourceLabel:    "统一目录树",
			LastTaskText:   "暂无任务",
			LastTaskTone:   "info",
			ColorLabel:     "无",
			Badges:         []string{},
			RiskTags:       []string{},
			Tags:           []string{},
			Endpoints:      []assetdto.EntryEndpoint{},
			Metadata:       []assetdto.MetadataRow{},
		},
	}, nil
}

func (fakeAssetService) UploadSelection(_ context.Context, _ string, request assetdto.UploadSelectionRequest) (assetdto.UploadSelectionResponse, error) {
	return assetdto.UploadSelectionResponse{
		Message:      "已上传文件",
		CreatedCount: len(request.Files),
	}, nil
}

func (fakeAssetService) UpdateAnnotations(_ context.Context, _ string, request assetdto.UpdateAnnotationsRequest) (assetdto.UpdateAnnotationsResponse, error) {
	_ = request
	return assetdto.UpdateAnnotationsResponse{Message: "资产标记已更新"}, nil
}

func (fakeAssetService) DeleteEntry(context.Context, string) (assetdto.DeleteEntryResponse, error) {
	return assetdto.DeleteEntryResponse{Message: "条目已删除"}, nil
}

func (fakeAssetService) BrowseLibrary(_ context.Context, libraryID string, _ assetdto.BrowseQuery) (assetdto.BrowseLibraryResponse, error) {
	return assetdto.BrowseLibraryResponse{
		Breadcrumbs: []assetdto.Breadcrumb{{ID: nil, Label: "商业摄影资产库"}},
		Items: []assetdto.EntryRecord{
			{
				ID:             "asset-1",
				LibraryID:      libraryID,
				Type:           "file",
				LifecycleState: "ACTIVE",
				Name:           "cover.jpg",
				FileKind:       "图片",
				DisplayType:    "JPEG 图片",
				ModifiedAt:     "2026-04-10 12:20",
				CreatedAt:      "2026-04-10 12:20",
				Size:           "1.2 MB",
				Path:           "商业摄影资产库 / 原片 / cover.jpg",
				SourceLabel:    "统一资产",
				LastTaskText:   "暂无任务",
				LastTaskTone:   "info",
				ColorLabel:     "无",
				Badges:         []string{},
				RiskTags:       []string{},
				Tags:           []string{},
				Endpoints: []assetdto.EntryEndpoint{
					{Name: "商业摄影原片库", State: "已同步", Tone: "success", LastSyncAt: "2026-04-10 12:20", EndpointType: "local"},
				},
				Metadata: []assetdto.MetadataRow{{Label: "逻辑路径", Value: "/原片/cover.jpg"}},
			},
		},
		Total:               1,
		CurrentPathChildren: 1,
		EndpointNames:       []string{"商业摄影原片库"},
	}, nil
}

func (fakeAssetService) LoadEntry(context.Context, string) (*assetdto.EntryRecord, error) {
	return &assetdto.EntryRecord{
		ID:             "asset-1",
		LibraryID:      "photo",
		Type:           "file",
		LifecycleState: "ACTIVE",
		Name:           "cover.jpg",
		FileKind:       "图片",
		DisplayType:    "JPEG 图片",
		ModifiedAt:     "2026-04-10 12:20",
		CreatedAt:      "2026-04-10 12:20",
		Size:           "1.2 MB",
		Path:           "商业摄影资产库 / 原片 / cover.jpg",
		SourceLabel:    "统一资产",
		LastTaskText:   "暂无任务",
		LastTaskTone:   "info",
		ColorLabel:     "无",
		Badges:         []string{},
		RiskTags:       []string{},
		Tags:           []string{},
		Endpoints: []assetdto.EntryEndpoint{
			{Name: "商业摄影原片库", State: "已同步", Tone: "success", LastSyncAt: "2026-04-10 12:20", EndpointType: "local"},
		},
		Metadata: []assetdto.MetadataRow{{Label: "逻辑路径", Value: "/原片/cover.jpg"}},
	}, nil
}

func (fakeAssetService) ScanDirectory(context.Context, string, assetdto.ScanDirectoryRequest) (assetdto.ScanDirectoryResponse, error) {
	return assetdto.ScanDirectoryResponse{Message: "当前目录扫描已完成"}, nil
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

func TestNasNodesRouteReturnsPersistedRecords(t *testing.T) {
	t.Parallel()

	router := NewRouter(Dependencies{
		Runtime:      fakeRuntimeService{},
		Agents:       fakeAgentService{},
		LocalNodes:   fakeLocalNodeService{},
		NasNodes:     fakeNASNodeService{},
		LocalFolders: fakeLocalFolderService{},
	})

	request := httptest.NewRequest(http.MethodGet, "/api/storage/nas-nodes", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", recorder.Code)
	}
}

func TestSaveNasNodeRouteAcceptsValidPayload(t *testing.T) {
	t.Parallel()

	router := NewRouter(Dependencies{
		Runtime:      fakeRuntimeService{},
		Agents:       fakeAgentService{},
		LocalNodes:   fakeLocalNodeService{},
		NasNodes:     fakeNASNodeService{},
		LocalFolders: fakeLocalFolderService{},
	})

	body, err := json.Marshal(storagedto.SaveNasNodeRequest{
		Name:     "影像 NAS 01",
		Address:  `\\192.168.10.20\media`,
		Username: "mare-sync",
		Password: "secret",
	})
	if err != nil {
		t.Fatalf("marshal nas payload: %v", err)
	}

	request := httptest.NewRequest(http.MethodPost, "/api/storage/nas-nodes", bytes.NewReader(body))
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", recorder.Code)
	}
}

func TestLibrariesRouteReturnsPersistedRecords(t *testing.T) {
	t.Parallel()

	router := NewRouter(Dependencies{
		Runtime:      fakeRuntimeService{},
		Agents:       fakeAgentService{},
		LocalNodes:   fakeLocalNodeService{},
		NasNodes:     fakeNASNodeService{},
		LocalFolders: fakeLocalFolderService{},
		Assets:       fakeAssetService{},
	})

	request := httptest.NewRequest(http.MethodGet, "/api/libraries", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", recorder.Code)
	}
}

func TestBrowseLibraryRouteAcceptsValidQuery(t *testing.T) {
	t.Parallel()

	router := NewRouter(Dependencies{
		Runtime:      fakeRuntimeService{},
		Agents:       fakeAgentService{},
		LocalNodes:   fakeLocalNodeService{},
		NasNodes:     fakeNASNodeService{},
		LocalFolders: fakeLocalFolderService{},
		Assets:       fakeAssetService{},
	})

	request := httptest.NewRequest(http.MethodGet, "/api/libraries/photo/browse?page=1&pageSize=20&sortValue=修改时间&statusFilter=全部&fileTypeFilter=全部", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", recorder.Code)
	}
}

func TestLoadFileEntryRouteReturnsDetail(t *testing.T) {
	t.Parallel()

	router := NewRouter(Dependencies{
		Runtime:      fakeRuntimeService{},
		Agents:       fakeAgentService{},
		LocalNodes:   fakeLocalNodeService{},
		NasNodes:     fakeNASNodeService{},
		LocalFolders: fakeLocalFolderService{},
		Assets:       fakeAssetService{},
	})

	request := httptest.NewRequest(http.MethodGet, "/api/file-entries/asset-1", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", recorder.Code)
	}
}

func TestCreateLibraryRouteAcceptsValidPayload(t *testing.T) {
	t.Parallel()

	router := NewRouter(Dependencies{
		Runtime:      fakeRuntimeService{},
		Agents:       fakeAgentService{},
		LocalNodes:   fakeLocalNodeService{},
		NasNodes:     fakeNASNodeService{},
		LocalFolders: fakeLocalFolderService{},
		Assets:       fakeAssetService{},
	})

	body, err := json.Marshal(assetdto.CreateLibraryRequest{Name: "全新资产库"})
	if err != nil {
		t.Fatalf("marshal library payload: %v", err)
	}

	request := httptest.NewRequest(http.MethodPost, "/api/libraries", bytes.NewReader(body))
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", recorder.Code)
	}
}

func TestCreateDirectoryRouteAcceptsValidPayload(t *testing.T) {
	t.Parallel()

	router := NewRouter(Dependencies{
		Runtime:      fakeRuntimeService{},
		Agents:       fakeAgentService{},
		LocalNodes:   fakeLocalNodeService{},
		NasNodes:     fakeNASNodeService{},
		LocalFolders: fakeLocalFolderService{},
		Assets:       fakeAssetService{},
	})

	parentID := "dir-root-photo"
	body, err := json.Marshal(assetdto.CreateDirectoryRequest{
		ParentID: &parentID,
		Name:     "新建目录",
	})
	if err != nil {
		t.Fatalf("marshal directory payload: %v", err)
	}

	request := httptest.NewRequest(http.MethodPost, "/api/libraries/photo/directories", bytes.NewReader(body))
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", recorder.Code)
	}
}

func TestDeleteEntryRouteReturnsSuccess(t *testing.T) {
	t.Parallel()

	router := NewRouter(Dependencies{
		Runtime:      fakeRuntimeService{},
		Agents:       fakeAgentService{},
		LocalNodes:   fakeLocalNodeService{},
		NasNodes:     fakeNASNodeService{},
		LocalFolders: fakeLocalFolderService{},
		Assets:       fakeAssetService{},
	})

	request := httptest.NewRequest(http.MethodDelete, "/api/file-entries/dir-new", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", recorder.Code)
	}
}

func TestUpdateAnnotationsRouteAcceptsValidPayload(t *testing.T) {
	t.Parallel()

	router := NewRouter(Dependencies{
		Runtime:      fakeRuntimeService{},
		Agents:       fakeAgentService{},
		LocalNodes:   fakeLocalNodeService{},
		NasNodes:     fakeNASNodeService{},
		LocalFolders: fakeLocalFolderService{},
		Assets:       fakeAssetService{},
	})

	body, err := json.Marshal(assetdto.UpdateAnnotationsRequest{
		Rating:     5,
		ColorLabel: "红标",
	})
	if err != nil {
		t.Fatalf("marshal annotations payload: %v", err)
	}

	request := httptest.NewRequest(http.MethodPatch, "/api/file-entries/asset-1/annotations", bytes.NewReader(body))
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", recorder.Code)
	}
}

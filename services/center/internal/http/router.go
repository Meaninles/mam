package httpapi

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"

	"mare/services/center/internal/agentregistry"
	apperrors "mare/services/center/internal/errors"
	"mare/services/center/internal/response"
	"mare/services/center/internal/runtime"
	storagedto "mare/shared/contracts/dto/storage"
)

type RuntimeService interface {
	Health() runtime.HealthPayload
	Ready(ctx context.Context) (runtime.ReadinessPayload, error)
	Status(ctx context.Context) (runtime.RuntimeStatusPayload, error)
}

type AgentService interface {
	Register(ctx context.Context, registration agentregistry.Registration) (agentregistry.Agent, error)
	Heartbeat(ctx context.Context, heartbeat agentregistry.Heartbeat) (agentregistry.Agent, error)
}

type LocalFolderService interface {
	ListLocalFolders(ctx context.Context) ([]storagedto.LocalFolderRecord, error)
	SaveLocalFolder(ctx context.Context, request storagedto.SaveLocalFolderRequest) (storagedto.SaveLocalFolderResponse, error)
	RunLocalFolderScan(ctx context.Context, ids []string) (storagedto.RunLocalFolderScanResponse, error)
	RunLocalFolderConnectionTest(ctx context.Context, ids []string) (storagedto.RunLocalFolderConnectionTestResponse, error)
	UpdateLocalFolderHeartbeat(ctx context.Context, ids []string, heartbeatPolicy string) (storagedto.UpdateHeartbeatResponse, error)
	LoadLocalFolderScanHistory(ctx context.Context, id string) (storagedto.LocalFolderScanHistoryResponse, error)
	DeleteLocalFolder(ctx context.Context, id string) (storagedto.DeleteLocalFolderResponse, error)
}

type LocalNodeService interface {
	ListLocalNodes(ctx context.Context) ([]storagedto.LocalNodeRecord, error)
	SaveLocalNode(ctx context.Context, request storagedto.SaveLocalNodeRequest) (storagedto.SaveLocalNodeResponse, error)
	RunLocalNodeConnectionTest(ctx context.Context, ids []string) (storagedto.RunLocalNodeConnectionTestResponse, error)
	DeleteLocalNode(ctx context.Context, id string) (storagedto.DeleteLocalNodeResponse, error)
}

type Dependencies struct {
	Logger       *slog.Logger
	Runtime      RuntimeService
	Agents       AgentService
	LocalNodes   LocalNodeService
	LocalFolders LocalFolderService
}

func NewRouter(deps Dependencies) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		response.WriteSuccess(w, http.StatusOK, deps.Runtime.Health())
	})

	mux.HandleFunc("GET /readyz", func(w http.ResponseWriter, r *http.Request) {
		payload, err := deps.Runtime.Ready(r.Context())
		if err != nil {
			writeError(deps.Logger, w, err)
			return
		}
		response.WriteSuccess(w, http.StatusOK, payload)
	})

	mux.HandleFunc("GET /api/runtime/status", func(w http.ResponseWriter, r *http.Request) {
		payload, err := deps.Runtime.Status(r.Context())
		if err != nil {
			writeError(deps.Logger, w, err)
			return
		}
		response.WriteSuccess(w, http.StatusOK, payload)
	})

	mux.HandleFunc("POST /agent/register", func(w http.ResponseWriter, r *http.Request) {
		var payload agentregistry.Registration
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			writeError(deps.Logger, w, apperrors.BadRequest("注册请求格式无效"))
			return
		}

		agent, err := deps.Agents.Register(r.Context(), payload)
		if err != nil {
			writeError(deps.Logger, w, err)
			return
		}

		response.WriteSuccess(w, http.StatusOK, agent)
	})

	mux.HandleFunc("POST /agent/heartbeat", func(w http.ResponseWriter, r *http.Request) {
		var payload agentregistry.Heartbeat
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			writeError(deps.Logger, w, apperrors.BadRequest("心跳请求格式无效"))
			return
		}

		agent, err := deps.Agents.Heartbeat(r.Context(), payload)
		if err != nil {
			writeError(deps.Logger, w, err)
			return
		}

		response.WriteSuccess(w, http.StatusOK, agent)
	})

	mux.HandleFunc("GET /api/storage/local-folders", func(w http.ResponseWriter, r *http.Request) {
		payload, err := deps.LocalFolders.ListLocalFolders(r.Context())
		if err != nil {
			writeError(deps.Logger, w, err)
			return
		}
		response.WriteSuccess(w, http.StatusOK, payload)
	})

	mux.HandleFunc("GET /api/storage/local-nodes", func(w http.ResponseWriter, r *http.Request) {
		payload, err := deps.LocalNodes.ListLocalNodes(r.Context())
		if err != nil {
			writeError(deps.Logger, w, err)
			return
		}
		response.WriteSuccess(w, http.StatusOK, payload)
	})

	mux.HandleFunc("POST /api/storage/local-nodes", func(w http.ResponseWriter, r *http.Request) {
		var payload storagedto.SaveLocalNodeRequest
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			writeError(deps.Logger, w, apperrors.BadRequest("本地文件夹保存请求格式无效"))
			return
		}

		result, err := deps.LocalNodes.SaveLocalNode(r.Context(), payload)
		if err != nil {
			writeError(deps.Logger, w, err)
			return
		}
		response.WriteSuccess(w, http.StatusOK, result)
	})

	mux.HandleFunc("POST /api/storage/local-nodes/connection-test", func(w http.ResponseWriter, r *http.Request) {
		var payload storagedto.RunLocalNodeConnectionTestRequest
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			writeError(deps.Logger, w, apperrors.BadRequest("本地文件夹连接测试请求格式无效"))
			return
		}

		result, err := deps.LocalNodes.RunLocalNodeConnectionTest(r.Context(), payload.IDs)
		if err != nil {
			writeError(deps.Logger, w, err)
			return
		}
		response.WriteSuccess(w, http.StatusOK, result)
	})

	mux.HandleFunc("DELETE /api/storage/local-nodes/{id}", func(w http.ResponseWriter, r *http.Request) {
		result, err := deps.LocalNodes.DeleteLocalNode(r.Context(), r.PathValue("id"))
		if err != nil {
			writeError(deps.Logger, w, err)
			return
		}
		response.WriteSuccess(w, http.StatusOK, result)
	})

	mux.HandleFunc("POST /api/storage/local-folders", func(w http.ResponseWriter, r *http.Request) {
		var payload storagedto.SaveLocalFolderRequest
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			writeError(deps.Logger, w, apperrors.BadRequest("挂载文件夹保存请求格式无效"))
			return
		}

		result, err := deps.LocalFolders.SaveLocalFolder(r.Context(), payload)
		if err != nil {
			writeError(deps.Logger, w, err)
			return
		}
		response.WriteSuccess(w, http.StatusOK, result)
	})

	mux.HandleFunc("POST /api/storage/local-folders/scan", func(w http.ResponseWriter, r *http.Request) {
		var payload storagedto.RunLocalFolderScanRequest
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			writeError(deps.Logger, w, apperrors.BadRequest("扫描请求格式无效"))
			return
		}

		result, err := deps.LocalFolders.RunLocalFolderScan(r.Context(), payload.IDs)
		if err != nil {
			writeError(deps.Logger, w, err)
			return
		}
		response.WriteSuccess(w, http.StatusOK, result)
	})

	mux.HandleFunc("POST /api/storage/local-folders/connection-test", func(w http.ResponseWriter, r *http.Request) {
		var payload storagedto.RunLocalFolderConnectionTestRequest
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			writeError(deps.Logger, w, apperrors.BadRequest("连接测试请求格式无效"))
			return
		}

		result, err := deps.LocalFolders.RunLocalFolderConnectionTest(r.Context(), payload.IDs)
		if err != nil {
			writeError(deps.Logger, w, err)
			return
		}
		response.WriteSuccess(w, http.StatusOK, result)
	})

	mux.HandleFunc("PATCH /api/storage/local-folders/heartbeat", func(w http.ResponseWriter, r *http.Request) {
		var payload storagedto.UpdateHeartbeatRequest
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			writeError(deps.Logger, w, apperrors.BadRequest("心跳更新请求格式无效"))
			return
		}

		result, err := deps.LocalFolders.UpdateLocalFolderHeartbeat(r.Context(), payload.IDs, payload.HeartbeatPolicy)
		if err != nil {
			writeError(deps.Logger, w, err)
			return
		}
		response.WriteSuccess(w, http.StatusOK, result)
	})

	mux.HandleFunc("GET /api/storage/local-folders/{id}/scan-history", func(w http.ResponseWriter, r *http.Request) {
		result, err := deps.LocalFolders.LoadLocalFolderScanHistory(r.Context(), r.PathValue("id"))
		if err != nil {
			writeError(deps.Logger, w, err)
			return
		}
		response.WriteSuccess(w, http.StatusOK, result)
	})

	mux.HandleFunc("DELETE /api/storage/local-folders/{id}", func(w http.ResponseWriter, r *http.Request) {
		result, err := deps.LocalFolders.DeleteLocalFolder(r.Context(), r.PathValue("id"))
		if err != nil {
			writeError(deps.Logger, w, err)
			return
		}
		response.WriteSuccess(w, http.StatusOK, result)
	})

	return withCORS(mux)
}

func writeError(logger *slog.Logger, w http.ResponseWriter, err error) {
	if logger != nil {
		logger.Error("request failed", slog.String("error", err.Error()))
	}

	statusCode, code, message := apperrors.ToHTTP(err)
	response.WriteError(w, statusCode, code, message)
}

func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

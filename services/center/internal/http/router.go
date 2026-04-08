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

type Dependencies struct {
	Logger  *slog.Logger
	Runtime RuntimeService
	Agents  AgentService
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
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

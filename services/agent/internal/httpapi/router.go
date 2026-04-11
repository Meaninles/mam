package httpapi

import (
	"context"
	"encoding/json"
	"net/http"

	importdto "mare/shared/contracts/dto/importing"
)

type ImportService interface {
	DiscoverSources(rctx context.Context) ([]importdto.SourceDescriptor, error)
	BrowseSource(rctx context.Context, request importdto.BrowseRequest) (importdto.BrowseResponse, error)
	ExecuteImport(rctx context.Context, request importdto.ExecuteImportRequest) (importdto.ExecuteImportResponse, error)
}

type Dependencies struct {
	Importing ImportService
}

type envelope[T any] struct {
	Data T `json:"data"`
}

func NewRouter(deps Dependencies) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	mux.HandleFunc("POST /api/import/sources/discover", func(w http.ResponseWriter, r *http.Request) {
		result, err := deps.Importing.DiscoverSources(r.Context())
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		writeJSON(w, http.StatusOK, importdto.DiscoverSourcesResponse{Sources: result})
	})

	mux.HandleFunc("POST /api/import/sources/browse", func(w http.ResponseWriter, r *http.Request) {
		var payload importdto.BrowseRequest
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		result, err := deps.Importing.BrowseSource(r.Context(), payload)
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		writeJSON(w, http.StatusOK, result)
	})

	mux.HandleFunc("POST /api/import/execute", func(w http.ResponseWriter, r *http.Request) {
		var payload importdto.ExecuteImportRequest
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		result, err := deps.Importing.ExecuteImport(r.Context(), payload)
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		writeJSON(w, http.StatusOK, result)
	})

	return mux
}

func writeJSON[T any](w http.ResponseWriter, status int, payload T) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(envelope[T]{Data: payload})
}

func writeError(w http.ResponseWriter, status int, err error) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"error": map[string]string{
			"message": err.Error(),
		},
	})
}

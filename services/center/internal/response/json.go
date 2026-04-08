package response

import (
	"encoding/json"
	"net/http"
	"time"
)

type SuccessEnvelope[T any] struct {
	Data      T      `json:"data"`
	Timestamp string `json:"timestamp"`
}

type ErrorEnvelope struct {
	Error     ErrorBody `json:"error"`
	Timestamp string    `json:"timestamp"`
}

type ErrorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func WriteSuccess[T any](w http.ResponseWriter, statusCode int, payload T) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(SuccessEnvelope[T]{
		Data:      payload,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	})
}

func WriteError(w http.ResponseWriter, statusCode int, code string, message string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(ErrorEnvelope{
		Error: ErrorBody{
			Code:    code,
			Message: message,
		},
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	})
}

package apperrors

import "net/http"

type AppError struct {
	Code       string
	Message    string
	StatusCode int
}

func (e *AppError) Error() string {
	return e.Message
}

func New(code string, message string, statusCode int) *AppError {
	return &AppError{
		Code:       code,
		Message:    message,
		StatusCode: statusCode,
	}
}

func BadRequest(message string) *AppError {
	return New("bad_request", message, http.StatusBadRequest)
}

func NotFound(message string) *AppError {
	return New("not_found", message, http.StatusNotFound)
}

func Internal(message string) *AppError {
	return New("internal_error", message, http.StatusInternalServerError)
}

func ToHTTP(err error) (int, string, string) {
	if appErr, ok := err.(*AppError); ok {
		return appErr.StatusCode, appErr.Code, appErr.Message
	}
	return http.StatusInternalServerError, "internal_error", "服务内部错误"
}

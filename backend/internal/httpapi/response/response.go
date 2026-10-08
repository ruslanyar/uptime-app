// Package response provides the shared JSON response format.
package response

import (
	"encoding/json"
	"net/http"
)

type ErrorResponse struct {
	Error ErrorDetail `json:"error" binding:"required"`
}

type ErrorDetail struct {
	Code    string `json:"code" binding:"required"`
	Message string `json:"message" binding:"required"`
}

func JSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	// Status is already committed; encoding failure cannot become a new response.
	_ = json.NewEncoder(w).Encode(v)
}
func Error(w http.ResponseWriter, status int, code, message string) {
	JSON(w, status, ErrorResponse{Error: ErrorDetail{Code: code, Message: message}})
}

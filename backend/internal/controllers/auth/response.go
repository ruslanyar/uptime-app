package authcontroller

import (
	"errors"
	"net/http"

	"uptime-app/backend/internal/auth"
	"uptime-app/backend/internal/httpapi/response"
)

func serviceError(w http.ResponseWriter, e error) {
	switch {
	case errors.Is(e, auth.ErrInvalid):
		response.Error(w, 400, "invalid_request", "Invalid input")
	case errors.Is(e, auth.ErrUnauthorized):
		response.Error(w, 401, "unauthorized", "Invalid credentials or token")
	case errors.Is(e, auth.ErrConflict):
		response.Error(w, 409, "email_conflict", "Email already registered")
	default:
		response.Error(w, 500, "internal_error", "Internal server error")
	}
}
func (a *Controller) result(w http.ResponseWriter, status int, result auth.Result, e error) {
	w.Header().Set("Cache-Control", "no-store")
	if e != nil {
		serviceError(w, e)
		return
	}
	a.cookie(w, result.RefreshToken, result.SessionExpires)
	response.JSON(w, status, result)
}

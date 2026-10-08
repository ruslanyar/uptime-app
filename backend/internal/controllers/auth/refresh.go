package authcontroller

import (
	"errors"
	"net/http"

	"uptime-app/backend/internal/auth"
)

// Refresh handles POST /auth/refresh.
// @Summary Refresh tokens
// @Description Requires refresh_token cookie. Rotates both cookies while preserving session expiry; invalid tokens clear cookies.
// @Tags auth
// @ID authRefresh
// @Produce json
// @Param X-CSRF-Protection header string true "CSRF protection" Enums(1)
// @Success 200 {object} authResponse
// @Header 200 {string} Cache-Control "no-store"
// @Header 200 {string} Set-Cookie "access_token and refresh_token HttpOnly cookies"
// @Failure 401 {object} response.ErrorResponse "Invalid credentials or token"
// @Failure 403 {object} response.ErrorResponse "Origin or CSRF rejected"
// @Failure 500 {object} response.ErrorResponse "Internal server error"
// @Router /auth/refresh [post]
func (a *Controller) Refresh(w http.ResponseWriter, r *http.Request) {
	result, e := a.service.Refresh(r.Context(), refreshCookie(r))
	// Transient failures must leave credentials available for recovery.
	if errors.Is(e, auth.ErrUnauthorized) {
		a.clearCookies(w)
	}
	a.result(w, 200, result, e)
}

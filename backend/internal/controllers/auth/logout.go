package authcontroller

import (
	"net/http"
)

// Logout handles POST /auth/logout.
// @Summary Log out
// @Description Uses refresh_token cookie when present. Idempotent; clears both cookies. Issued access JWTs remain valid.
// @Tags auth
// @ID authLogout
// @Produce json
// @Param X-CSRF-Protection header string true "CSRF protection" Enums(1)
// @Success 204 "No Content"
// @Header 204 {string} Cache-Control "no-store"
// @Header 204 {string} Set-Cookie "access_token and refresh_token HttpOnly cookies"
// @Failure 403 {object} response.ErrorResponse "Origin or CSRF rejected"
// @Failure 500 {object} response.ErrorResponse "Internal server error"
// @Router /auth/logout [post]
func (a *Controller) Logout(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if e := a.service.Logout(r.Context(), refreshCookie(r)); e != nil {
		serviceError(w, e)
		return
	}
	a.clearCookies(w)
	w.WriteHeader(204)
}

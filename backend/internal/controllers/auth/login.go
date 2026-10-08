package authcontroller

import (
	"net/http"
	"uptime-app/backend/internal/httpapi/request"

	"uptime-app/backend/internal/auth"
)

// Login handles POST /auth/login.
// @Summary Log in
// @Description Sets HttpOnly access_token and refresh_token cookies.
// @Tags auth
// @ID authLogin
// @Produce json
// @Accept json
// @Param body body loginRequest true "Request body"
// @Param X-CSRF-Protection header string true "CSRF protection" Enums(1)
// @Success 200 {object} authResponse
// @Header 200 {string} Cache-Control "no-store"
// @Header 200 {string} Set-Cookie "access_token and refresh_token HttpOnly cookies"
// @Failure 400 {object} response.ErrorResponse "Invalid input"
// @Failure 401 {object} response.ErrorResponse "Invalid credentials or token"
// @Failure 403 {object} response.ErrorResponse "Origin or CSRF rejected"
// @Failure 500 {object} response.ErrorResponse "Internal server error"
// @Router /auth/login [post]
func (a *Controller) Login(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	var v loginRequest
	if e := request.Decode(w, r, &v); e != nil {
		serviceError(w, e)
		return
	}
	if v.Email == "" || v.Password == "" {
		serviceError(w, auth.ErrInvalid)
		return
	}
	result, e := a.service.Login(r.Context(), v.Email, v.Password)
	a.result(w, 200, result, e)
}

package authcontroller

import (
	"net/http"
	"uptime-app/backend/internal/httpapi/request"
)

// Register handles POST /auth/register.
// @Summary Register an account
// @Description Registers an account and sets HttpOnly access_token and refresh_token cookies.
// @Tags auth
// @ID authRegister
// @Produce json
// @Accept json
// @Param body body registerRequest true "Request body"
// @Param X-CSRF-Protection header string true "CSRF protection" Enums(1)
// @Success 201 {object} authResponse
// @Header 201 {string} Cache-Control "no-store"
// @Header 201 {string} Set-Cookie "access_token and refresh_token HttpOnly cookies"
// @Failure 400 {object} response.ErrorResponse "Invalid input"
// @Failure 403 {object} response.ErrorResponse "Origin or CSRF rejected"
// @Failure 409 {object} response.ErrorResponse "Email already registered"
// @Failure 500 {object} response.ErrorResponse "Internal server error"
// @Router /auth/register [post]
func (a *Controller) Register(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	var v registerRequest
	if e := request.Decode(w, r, &v); e != nil {
		serviceError(w, e)
		return
	}
	result, e := a.service.Register(r.Context(), v.Name, v.Email, v.Password)
	a.result(w, 201, result, e)
}

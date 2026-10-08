package authcontroller

import (
	"net/http"
	"uptime-app/backend/internal/httpapi/request"

	"uptime-app/backend/internal/httpapi/response"
)

// Me handles GET /auth/me.
// @Summary Get current user
// @Description Accepts access_token cookie or Authorization: Bearer JWT. A present Authorization header takes precedence.
// @Tags auth
// @ID authMe
// @Produce json
// @Security BearerAuth
// @Success 200 {object} auth.User
// @Header 200 {string} Cache-Control "no-store"
// @Failure 401 {object} response.ErrorResponse "Invalid credentials or token"
// @Failure 403 {object} response.ErrorResponse "Origin or CSRF rejected"
// @Failure 500 {object} response.ErrorResponse "Internal server error"
// @Router /auth/me [get]
func (a *Controller) Me(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	raw, e := request.AccessToken(r)
	if e != nil {
		serviceError(w, e)
		return
	}
	u, e := a.service.Me(r.Context(), raw)
	if e != nil {
		serviceError(w, e)
		return
	}
	response.JSON(w, 200, u)
}

package authcontroller

import (
	"net/http"
	"strings"

	"uptime-app/backend/internal/auth"
	"uptime-app/backend/internal/httpapi/response"
)

func (a *Controller) Me(w http.ResponseWriter, r *http.Request) {
	fields := strings.Fields(r.Header.Get("Authorization"))
	if len(fields) != 2 || !strings.EqualFold(fields[0], "Bearer") {
		serviceError(w, auth.ErrUnauthorized)
		return
	}
	u, e := a.service.Me(r.Context(), fields[1])
	if e != nil {
		serviceError(w, e)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	response.JSON(w, 200, u)
}

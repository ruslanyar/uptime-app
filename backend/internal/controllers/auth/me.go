package authcontroller

import (
	"net/http"
	"uptime-app/backend/internal/httpapi/request"

	"uptime-app/backend/internal/httpapi/response"
)

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

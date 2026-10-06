package authcontroller

import (
	"net/http"
	"uptime-app/backend/internal/httpapi/response"
)

func (a *Controller) UpdateProfile(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	raw, e := accessToken(r)
	if e != nil {
		serviceError(w, e)
		return
	}
	var v struct {
		Name string `json:"name"`
	}
	if e = decode(w, r, &v); e != nil {
		serviceError(w, e)
		return
	}
	u, e := a.service.UpdateProfile(r.Context(), raw, v.Name)
	if e != nil {
		serviceError(w, e)
		return
	}
	response.JSON(w, 200, authResponse{User: u})
}

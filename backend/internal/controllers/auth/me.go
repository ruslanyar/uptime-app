package authcontroller

import (
	"net/http"
	"strings"

	"uptime-app/backend/internal/auth"
	"uptime-app/backend/internal/httpapi/response"
)

func (a *Controller) Me(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	var raw string
	if headers, present := r.Header["Authorization"]; present {
		fields := strings.Fields(r.Header.Get("Authorization"))
		if len(headers) != 1 || len(fields) != 2 || !strings.EqualFold(fields[0], "Bearer") {
			serviceError(w, auth.ErrUnauthorized)
			return
		}
		raw = fields[1]
	} else if cookie, err := r.Cookie("access_token"); err == nil {
		raw = cookie.Value
	}
	if raw == "" {
		serviceError(w, auth.ErrUnauthorized)
		return
	}
	u, e := a.service.Me(r.Context(), raw)
	if e != nil {
		serviceError(w, e)
		return
	}
	response.JSON(w, 200, u)
}

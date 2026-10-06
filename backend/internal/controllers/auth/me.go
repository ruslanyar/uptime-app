package authcontroller

import (
	"net/http"
	"strings"

	"uptime-app/backend/internal/auth"
	"uptime-app/backend/internal/httpapi/response"
)

func (a *Controller) Me(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	raw, e := accessToken(r)
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

func accessToken(r *http.Request) (string, error) {
	if headers, present := r.Header["Authorization"]; present {
		fields := strings.Fields(r.Header.Get("Authorization"))
		if len(headers) != 1 || len(fields) != 2 || !strings.EqualFold(fields[0], "Bearer") {
			return "", auth.ErrUnauthorized
		}
		return fields[1], nil
	}
	if cookie, e := r.Cookie("access_token"); e == nil && cookie.Value != "" {
		return cookie.Value, nil
	}
	return "", auth.ErrUnauthorized
}

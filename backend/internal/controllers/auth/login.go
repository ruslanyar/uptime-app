package authcontroller

import (
	"net/http"
	"uptime-app/backend/internal/httpapi/request"

	"uptime-app/backend/internal/auth"
)

func (a *Controller) Login(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	var v struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
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

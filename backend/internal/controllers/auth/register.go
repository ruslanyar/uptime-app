package authcontroller

import (
	"net/http"
)

func (a *Controller) Register(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	var v struct {
		Name     string `json:"name"`
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if e := decode(w, r, &v); e != nil {
		serviceError(w, e)
		return
	}
	result, e := a.service.Register(r.Context(), v.Name, v.Email, v.Password)
	a.result(w, 201, result, e)
}

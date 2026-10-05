package authcontroller

import (
	"net/http"
)

func (a *Controller) Logout(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if e := a.service.Logout(r.Context(), refreshCookie(r)); e != nil {
		serviceError(w, e)
		return
	}
	a.clearCookies(w)
	w.WriteHeader(204)
}

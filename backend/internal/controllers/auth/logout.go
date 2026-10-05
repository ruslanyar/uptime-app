package authcontroller

import (
	"net/http"
	"time"
)

func (a *Controller) Logout(w http.ResponseWriter, r *http.Request) {
	if e := a.service.Logout(r.Context(), refreshCookie(r)); e != nil {
		serviceError(w, e)
		return
	}
	a.cookie(w, "", time.Time{})
	w.WriteHeader(204)
}

package authcontroller

import (
	"net/http"
)

func (a *Controller) Refresh(w http.ResponseWriter, r *http.Request) {
	result, e := a.service.Refresh(r.Context(), refreshCookie(r))
	a.result(w, 200, result, e)
}

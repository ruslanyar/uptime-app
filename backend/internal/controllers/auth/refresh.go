package authcontroller

import (
	"errors"
	"net/http"

	"uptime-app/backend/internal/auth"
)

func (a *Controller) Refresh(w http.ResponseWriter, r *http.Request) {
	result, e := a.service.Refresh(r.Context(), refreshCookie(r))
	if errors.Is(e, auth.ErrUnauthorized) {
		a.clearCookies(w)
	}
	a.result(w, 200, result, e)
}

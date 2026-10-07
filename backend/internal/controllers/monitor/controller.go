// Package monitorcontroller handles monitor creation requests.
package monitorcontroller

import (
	"context"
	"errors"
	"net/http"
	"uptime-app/backend/internal/auth"
	"uptime-app/backend/internal/httpapi/request"
	"uptime-app/backend/internal/httpapi/response"
	"uptime-app/backend/internal/monitor"
)

type AuthService interface {
	Me(context.Context, string) (auth.User, error)
}
type Service interface {
	Create(context.Context, string, string, int32) (monitor.Monitor, error)
}
type Controller struct {
	auth    AuthService
	service Service
}

func New(auth AuthService, service Service) *Controller {
	return &Controller{auth: auth, service: service}
}
func failure(w http.ResponseWriter, e error) {
	switch {
	case errors.Is(e, auth.ErrUnauthorized):
		response.Error(w, 401, "unauthorized", "Invalid credentials or token")
	case errors.Is(e, monitor.ErrInvalid), errors.Is(e, auth.ErrInvalid):
		response.Error(w, 400, "invalid_request", "Invalid input")
	case errors.Is(e, monitor.ErrConflict):
		response.Error(w, 409, "monitor_conflict", "Monitor already exists")
	default:
		response.Error(w, 500, "internal_error", "Internal server error")
	}
}
func (c *Controller) Create(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	raw, e := request.AccessToken(r)
	if e != nil {
		failure(w, e)
		return
	}
	u, e := c.auth.Me(r.Context(), raw)
	if e != nil {
		failure(w, e)
		return
	}
	var input struct {
		URL             string `json:"url"`
		IntervalSeconds int32  `json:"interval_seconds"`
	}
	if e = request.Decode(w, r, &input); e != nil {
		failure(w, e)
		return
	}
	m, e := c.service.Create(r.Context(), u.ID, input.URL, input.IntervalSeconds)
	if e != nil {
		failure(w, e)
		return
	}
	response.JSON(w, http.StatusCreated, m)
}

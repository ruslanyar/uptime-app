// Package monitorcontroller handles monitor management requests.
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
	List(context.Context, string) ([]monitor.Monitor, error)
	Update(context.Context, string, string, string, int32) (monitor.Monitor, error)
	Delete(context.Context, string, string) error
}

// Controller delegates cancellation and deadlines to AuthService and Service.
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
	case errors.Is(e, monitor.ErrNotFound):
		response.Error(w, http.StatusNotFound, "monitor_not_found", "Monitor not found")
	case errors.Is(e, monitor.ErrConflict):
		response.Error(w, 409, "monitor_conflict", "Monitor already exists")
	default:
		// Storage errors can contain internal details; expose only the public message.
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

func (c *Controller) List(w http.ResponseWriter, r *http.Request) {
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
	items, e := c.service.List(r.Context(), u.ID)
	if e != nil {
		failure(w, e)
		return
	}
	if items == nil {
		// The API contract requires an array even when the service returns nil.
		items = []monitor.Monitor{}
	}
	response.JSON(w, http.StatusOK, struct {
		Monitors []monitor.Monitor `json:"monitors"`
	}{items})
}

func (c *Controller) Update(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	raw, err := request.AccessToken(r)
	if err != nil {
		failure(w, err)
		return
	}
	user, err := c.auth.Me(r.Context(), raw)
	if err != nil {
		failure(w, err)
		return
	}
	var input struct {
		URL             string `json:"url"`
		IntervalSeconds int32  `json:"interval_seconds"`
	}
	if err = request.Decode(w, r, &input); err != nil {
		failure(w, err)
		return
	}
	m, err := c.service.Update(r.Context(), user.ID, r.PathValue("id"), input.URL, input.IntervalSeconds)
	if err != nil {
		failure(w, err)
		return
	}
	response.JSON(w, http.StatusOK, m)
}

func (c *Controller) Delete(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	raw, err := request.AccessToken(r)
	if err != nil {
		failure(w, err)
		return
	}
	user, err := c.auth.Me(r.Context(), raw)
	if err != nil {
		failure(w, err)
		return
	}
	if err = c.service.Delete(r.Context(), user.ID, r.PathValue("id")); err != nil {
		failure(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

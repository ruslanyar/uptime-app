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

// Create handles POST /monitors.
// @Summary Create a monitor
// @Description Accepts access_token cookie or Authorization: Bearer JWT. A present Authorization header takes precedence. URL must use HTTP(S), without credentials or fragments.
// @Tags monitor
// @ID monitorCreate
// @Produce json
// @Accept json
// @Param body body monitorRequest true "Request body"
// @Param X-CSRF-Protection header string true "CSRF protection" Enums(1)
// @Security BearerAuth
// @Success 201 {object} monitor.Monitor
// @Header 201 {string} Cache-Control "no-store"
// @Failure 400 {object} response.ErrorResponse "Invalid input"
// @Failure 401 {object} response.ErrorResponse "Invalid credentials or token"
// @Failure 403 {object} response.ErrorResponse "Origin or CSRF rejected"
// @Failure 409 {object} response.ErrorResponse "Monitor URL already exists"
// @Failure 500 {object} response.ErrorResponse "Internal server error"
// @Router /monitors [post]
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
	var input monitorRequest
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

// List handles GET /monitors.
// @Summary List monitors
// @Description Accepts access_token cookie or Authorization: Bearer JWT. A present Authorization header takes precedence. Returns an empty array when there are no monitors.
// @Tags monitor
// @ID monitorList
// @Produce json
// @Security BearerAuth
// @Success 200 {object} listResponse
// @Header 200 {string} Cache-Control "no-store"
// @Failure 400 {object} response.ErrorResponse "Invalid input"
// @Failure 401 {object} response.ErrorResponse "Invalid credentials or token"
// @Failure 403 {object} response.ErrorResponse "Origin or CSRF rejected"
// @Failure 500 {object} response.ErrorResponse "Internal server error"
// @Router /monitors [get]
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
	response.JSON(w, http.StatusOK, listResponse{Monitors: items})
}

// Update handles PUT /monitors/{id}.
// @Summary Update a monitor
// @Description Accepts access_token cookie or Authorization: Bearer JWT. A present Authorization header takes precedence. URL must use HTTP(S), without credentials or fragments.
// @Tags monitor
// @ID monitorUpdate
// @Produce json
// @Accept json
// @Param body body monitorRequest true "Request body"
// @Param X-CSRF-Protection header string true "CSRF protection" Enums(1)
// @Param id path string true "Monitor UUID" Format(uuid)
// @Security BearerAuth
// @Success 200 {object} monitor.Monitor
// @Header 200 {string} Cache-Control "no-store"
// @Failure 400 {object} response.ErrorResponse "Invalid input"
// @Failure 401 {object} response.ErrorResponse "Invalid credentials or token"
// @Failure 403 {object} response.ErrorResponse "Origin or CSRF rejected"
// @Failure 404 {object} response.ErrorResponse "Monitor not found"
// @Failure 409 {object} response.ErrorResponse "Monitor URL already exists"
// @Failure 500 {object} response.ErrorResponse "Internal server error"
// @Router /monitors/{id} [put]
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
	var input monitorRequest
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

// Delete handles DELETE /monitors/{id}.
// @Summary Delete a monitor
// @Description Accepts access_token cookie or Authorization: Bearer JWT. A present Authorization header takes precedence.
// @Tags monitor
// @ID monitorDelete
// @Produce json
// @Param X-CSRF-Protection header string true "CSRF protection" Enums(1)
// @Param id path string true "Monitor UUID" Format(uuid)
// @Security BearerAuth
// @Success 204 "No Content"
// @Header 204 {string} Cache-Control "no-store"
// @Failure 400 {object} response.ErrorResponse "Invalid input"
// @Failure 401 {object} response.ErrorResponse "Invalid credentials or token"
// @Failure 403 {object} response.ErrorResponse "Origin or CSRF rejected"
// @Failure 404 {object} response.ErrorResponse "Monitor not found"
// @Failure 500 {object} response.ErrorResponse "Internal server error"
// @Router /monitors/{id} [delete]
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

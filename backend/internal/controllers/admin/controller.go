// Package admincontroller handles administrator-only user management.
package admincontroller

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"uptime-app/backend/internal/admin"
	"uptime-app/backend/internal/auth"
	"uptime-app/backend/internal/avatar"
	"uptime-app/backend/internal/httpapi/request"
	"uptime-app/backend/internal/httpapi/response"
)

type AuthService interface {
	Me(context.Context, string) (auth.User, error)
}
type Service interface {
	List(context.Context, int32) (admin.List, error)
	Delete(context.Context, string, string) (string, error)
}
type Controller struct {
	auth      AuthService
	service   Service
	avatarDir string
}

func New(auth AuthService, service Service, avatarDir string) *Controller {
	return &Controller{auth: auth, service: service, avatarDir: avatarDir}
}

func failure(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, auth.ErrUnauthorized):
		response.Error(w, 401, "unauthorized", "Invalid credentials or token")
	case errors.Is(err, admin.ErrForbidden):
		response.Error(w, 403, "forbidden", "Administrator role required")
	case errors.Is(err, auth.ErrInvalid):
		response.Error(w, 400, "invalid_request", "Invalid input")
	case errors.Is(err, admin.ErrNotFound):
		response.Error(w, 404, "user_not_found", "User not found")
	case errors.Is(err, admin.ErrProtected):
		response.Error(w, 409, "user_protected", "Administrator cannot be deleted")
	default:
		response.Error(w, 500, "internal_error", "Internal server error")
	}
}

func (c *Controller) administrator(w http.ResponseWriter, r *http.Request) (auth.User, bool) {
	w.Header().Set("Cache-Control", "no-store")
	raw, err := request.AccessToken(r)
	if err != nil {
		failure(w, err)
		return auth.User{}, false
	}
	user, err := c.auth.Me(r.Context(), raw)
	if err != nil {
		failure(w, err)
		return auth.User{}, false
	}
	if user.Role != "admin" {
		failure(w, admin.ErrForbidden)
		return auth.User{}, false
	}
	return user, true
}

// List handles GET /admin/users.
// @Summary List registered users
// @Description Requires current admin role in PostgreSQL. Accepts access_token cookie or Bearer JWT; a present Authorization header takes precedence. Sorted by email and ID, 25 users per page. Out-of-range pages return an empty array.
// @Tags admin
// @ID adminUsersList
// @Produce json
// @Param page query int false "Page number" minimum(1) default(1)
// @Security BearerAuth
// @Success 200 {object} admin.List
// @Header 200 {string} Cache-Control "no-store"
// @Failure 400 {object} response.ErrorResponse "Invalid page"
// @Failure 401 {object} response.ErrorResponse "Invalid credentials or token"
// @Failure 403 {object} response.ErrorResponse "Admin role required or Origin rejected"
// @Failure 500 {object} response.ErrorResponse "Internal server error"
// @Router /admin/users [get]
func (c *Controller) List(w http.ResponseWriter, r *http.Request) {
	if _, ok := c.administrator(w, r); !ok {
		return
	}
	page := int64(1)
	if values, present := r.URL.Query()["page"]; present {
		if len(values) != 1 {
			failure(w, auth.ErrInvalid)
			return
		}
		var err error
		page, err = strconv.ParseInt(values[0], 10, 32)
		if err != nil || page < 1 {
			failure(w, auth.ErrInvalid)
			return
		}
	}
	result, err := c.service.List(r.Context(), int32(page))
	if err != nil {
		failure(w, err)
		return
	}
	response.JSON(w, http.StatusOK, result)
}

// Delete handles DELETE /admin/users/{id}.
// @Summary Permanently delete a regular user
// @Description Requires current admin role in PostgreSQL. Accepts access_token cookie or Bearer JWT; a present Authorization header takes precedence. Deletes monitors, sessions and refresh tokens by cascade. Admin accounts, including the caller, are protected.
// @Tags admin
// @ID adminUserDelete
// @Produce json
// @Param id path string true "User UUID" Format(uuid)
// @Param X-CSRF-Protection header string true "CSRF protection" Enums(1)
// @Security BearerAuth
// @Success 204 "No Content"
// @Header 204 {string} Cache-Control "no-store"
// @Failure 400 {object} response.ErrorResponse "Invalid UUID"
// @Failure 401 {object} response.ErrorResponse "Invalid credentials or token"
// @Failure 403 {object} response.ErrorResponse "Admin role required or Origin/CSRF rejected"
// @Failure 404 {object} response.ErrorResponse "User not found"
// @Failure 409 {object} response.ErrorResponse "Admin account protected"
// @Failure 500 {object} response.ErrorResponse "Internal server error"
// @Router /admin/users/{id} [delete]
func (c *Controller) Delete(w http.ResponseWriter, r *http.Request) {
	actor, ok := c.administrator(w, r)
	if !ok {
		return
	}
	avatarURL, err := c.service.Delete(r.Context(), actor.ID, r.PathValue("id"))
	if err != nil {
		failure(w, err)
		return
	}
	// Database commit must precede best-effort file cleanup.
	avatar.Remove(c.avatarDir, avatarURL)
	w.WriteHeader(http.StatusNoContent)
}

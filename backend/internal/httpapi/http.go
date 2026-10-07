// Package httpapi wires API routes and request protection.
package httpapi

import (
	"net/http"
	"uptime-app/backend/internal/avatar"

	"uptime-app/backend/internal/config"
	authcontroller "uptime-app/backend/internal/controllers/auth"
	monitorcontroller "uptime-app/backend/internal/controllers/monitor"
)

// Service is the authentication contract used by the HTTP API.
type Service = authcontroller.Service

func New(service Service, monitors monitorcontroller.Service, cfg config.Config) http.Handler {
	controller := authcontroller.New(service, cfg)
	mux := http.NewServeMux()
	monitorController := monitorcontroller.New(service, monitors)
	mux.HandleFunc("POST /api/v1/monitors", monitorController.Create)
	mux.HandleFunc("GET /api/v1/monitors", monitorController.List)
	mux.HandleFunc("PUT /api/v1/monitors/{id}", monitorController.Update)
	mux.HandleFunc("DELETE /api/v1/monitors/{id}", monitorController.Delete)
	mux.HandleFunc("POST /api/v1/auth/register", controller.Register)
	mux.HandleFunc("POST /api/v1/auth/login", controller.Login)
	mux.HandleFunc("POST /api/v1/auth/refresh", controller.Refresh)
	mux.HandleFunc("POST /api/v1/auth/logout", controller.Logout)
	mux.HandleFunc("GET /api/v1/auth/me", controller.Me)
	mux.HandleFunc("POST /api/v1/auth/profile", controller.UpdateProfile)
	mux.HandleFunc("GET /api/v1/avatars/{filename}", avatar.Serve(cfg.AvatarDir))
	return protect(cfg, mux)
}

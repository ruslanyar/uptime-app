// Package authcontroller handles authentication HTTP requests.
package authcontroller

import (
	"context"

	"uptime-app/backend/internal/auth"
	"uptime-app/backend/internal/config"
)

type Service interface {
	Register(context.Context, string, string, string) (auth.Result, error)
	Login(context.Context, string, string) (auth.Result, error)
	Refresh(context.Context, string) (auth.Result, error)
	Logout(context.Context, string) error
	Me(context.Context, string) (auth.User, error)
	UpdateProfile(context.Context, string, string, ...string) (auth.User, error)
}

// Controller forwards the request context to Service; avatar file operations
// do not observe cancellation.
type Controller struct {
	service Service
	cfg     config.Config
}

func New(service Service, cfg config.Config) *Controller {
	return &Controller{service: service, cfg: cfg}
}

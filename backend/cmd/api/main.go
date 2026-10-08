package main

//go:generate sh ../../scripts/generate-openapi.sh

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"uptime-app/backend/internal/app"
	"uptime-app/backend/internal/config"
)

func run() error {
	cfg, e := config.Load()
	if e != nil {
		return e
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	return app.Run(ctx, cfg)
}

// @title Uptime API
// @version 1.0
// @description Uptime API. JSON bodies accept one object, reject unknown fields and are limited to 16 KiB. Requests with Origin require an allowed origin. Mutations require X-CSRF-Protection: 1.
// @BasePath /api/v1
// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
// @description JWT with the Bearer prefix. Access-cookie is also accepted; a present Authorization header takes precedence even if invalid. Cookie authentication is described per operation because OpenAPI 2.0 has no cookie security scheme.
func main() {
	if e := run(); e != nil {
		fmt.Fprintln(os.Stderr, e)
		os.Exit(1)
	}
}

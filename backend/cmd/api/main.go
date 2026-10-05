package main

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
func main() {
	if e := run(); e != nil {
		fmt.Fprintln(os.Stderr, e)
		os.Exit(1)
	}
}

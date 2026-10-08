package app

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"uptime-app/backend/internal/auth"
	"uptime-app/backend/internal/config"
	"uptime-app/backend/internal/httpapi"
	"uptime-app/backend/internal/monitor"
	"uptime-app/backend/internal/storage/postgres"
)

// Run uses ctx for startup and graceful shutdown, not as the base request context.
// Clean shutdown after cancellation returns nil; startup, serving or shutdown
// failures return errors. Failed graceful shutdown forcibly closes connections.
func Run(ctx context.Context, cfg config.Config) error {
	pool, e := pgxpool.New(ctx, cfg.DatabaseURL)
	if e != nil {
		return fmt.Errorf("configure PostgreSQL: %w", e)
	}
	defer pool.Close()
	startup, cancel := context.WithTimeout(ctx, 10*time.Second)
	e = pool.Ping(startup)
	cancel()
	if e != nil {
		return fmt.Errorf("connect to PostgreSQL: %w", e)
	}
	service, e := auth.NewService(postgres.New(pool), auth.NewTokens(cfg.JWTSecret, cfg.JWTIssuer, cfg.JWTAudience))
	if e != nil {
		return e
	}
	server := &http.Server{Addr: cfg.HTTPAddr, Handler: httpapi.New(service, monitor.NewService(postgres.New(pool)), cfg), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 30 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 16 * 1024}
	// Let the server goroutine finish even when shutdown stops waiting on done.
	done := make(chan error, 1)
	go func() { done <- server.ListenAndServe() }()
	select {
	case e = <-done:
		if errors.Is(e, http.ErrServerClosed) {
			return nil
		}
		return e
	case <-ctx.Done():
	}
	// Shutdown needs its own deadline because ctx is already canceled.
	shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if e = server.Shutdown(shutdown); e != nil {
		_ = server.Close()
		return e
	}
	return nil
}

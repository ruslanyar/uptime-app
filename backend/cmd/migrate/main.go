package main

import (
	"context"
	"fmt"
	"github.com/jackc/pgx/v5"
	"os"
	"os/signal"
	"syscall"
	"time"
	"uptime-app/backend/internal/config"
	"uptime-app/backend/internal/migrations"
)

func run() error {
	if len(os.Args) != 2 {
		return fmt.Errorf("usage: migrate up|status|down")
	}
	get, err := config.Environment(os.Getenv("APP_ENV"))
	if err != nil {
		return err
	}
	url := get("DATABASE_URL")
	if url == "" {
		return fmt.Errorf("DATABASE_URL is required")
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	conn, e := pgx.Connect(ctx, url)
	if e != nil {
		return fmt.Errorf("connect to PostgreSQL: %w", e)
	}
	defer func() {
		// A fresh context lets cleanup release session locks after signal cancellation.
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = conn.Close(ctx)
	}()
	status, e := migrations.Run(ctx, conn, os.DirFS("migrations"), os.Args[1])
	if e != nil {
		return e
	}
	fmt.Printf("current=%d target=%d\n", status.Current, status.Target)
	return nil
}
func main() {
	if e := run(); e != nil {
		fmt.Fprintln(os.Stderr, e)
		os.Exit(1)
	}
}

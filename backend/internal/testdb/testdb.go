// Package testdb provisions disposable databases for PostgreSQL integration tests.
package testdb

import (
	"context"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"net/url"
	"strings"
	"testing"
	"time"
	"uptime-app/backend/internal/auth"
	"uptime-app/backend/internal/config"
)

func New(t *testing.T) (context.Context, *pgxpool.Pool, string) {
	t.Helper()
	get, err := config.Environment("test")
	if err != nil {
		t.Fatal(err)
	}
	databaseURL := get("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not set; PostgreSQL integration NOT verified")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	t.Cleanup(cancel)
	admin, e := pgx.Connect(ctx, databaseURL)
	if e != nil {
		t.Fatal(e)
	}
	name := "auth_test_" + strings.ReplaceAll(auth.NewID(), "-", "")
	if _, e = admin.Exec(ctx, "CREATE DATABASE "+pgx.Identifier{name}.Sanitize()); e != nil {
		_ = admin.Close(ctx)
		t.Fatal(e)
	}
	cfg, e := pgxpool.ParseConfig(databaseURL)
	if e != nil {
		t.Fatal(e)
	}
	cfg.ConnConfig.Database = name
	pool, e := pgxpool.NewWithConfig(ctx, cfg)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() {
		pool.Close()
		cleanup, stop := context.WithTimeout(context.Background(), 10*time.Second)
		defer stop()
		_, e := admin.Exec(cleanup, "DROP DATABASE "+pgx.Identifier{name}.Sanitize()+" WITH (FORCE)")
		if e != nil {
			t.Error(e)
		}
		_ = admin.Close(cleanup)
	})
	parsed, e := url.Parse(databaseURL)
	if e != nil {
		t.Fatal(e)
	}
	parsed.Path = "/" + name
	return ctx, pool, parsed.String()
}

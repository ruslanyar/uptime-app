package main

import (
	"context"
	"flag"
	"fmt"
	"github.com/jackc/pgx/v5/pgxpool"
	"io"
	"os"
	"time"
	"uptime-app/backend/internal/admin"
	"uptime-app/backend/internal/config"
	"uptime-app/backend/internal/storage/postgres"
)

func run(args []string, output io.Writer) error {
	flags := flag.NewFlagSet("admin grant", flag.ContinueOnError)
	email := flags.String("email", "", "existing user's email")
	if len(args) == 0 || args[0] != "grant" {
		return fmt.Errorf("usage: admin grant --email <email>")
	}
	if err := flags.Parse(args[1:]); err != nil {
		return err
	}
	if *email == "" || flags.NArg() != 0 {
		return fmt.Errorf("usage: admin grant --email <email>")
	}
	get, err := config.Environment(os.Getenv("APP_ENV"))
	if err != nil {
		return err
	}
	if get("DATABASE_URL") == "" {
		return fmt.Errorf("DATABASE_URL is required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, get("DATABASE_URL"))
	if err != nil {
		return fmt.Errorf("configure PostgreSQL: %w", err)
	}
	defer pool.Close()
	if err := admin.NewService(postgres.New(pool)).Grant(ctx, *email); err != nil {
		return err
	}
	fmt.Fprintln(output, "Administrator role granted.")
	return nil
}

func main() {
	if err := run(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

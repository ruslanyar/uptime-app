package migrations_test

import (
	"github.com/jackc/pgx/v5"
	"os"
	"testing"
	"testing/fstest"
	"uptime-app/backend/internal/migrations"
	"uptime-app/backend/internal/testdb"
)

func TestUserRoleMigration(t *testing.T) {
	ctx, pool, url := testdb.New(t)
	conn, err := pgx.Connect(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(ctx)
	before := fstest.MapFS{}
	for _, name := range []string{"00001_create_auth_tables.sql", "00002_add_user_avatar.sql", "00003_create_monitors.sql", "00004_add_monitor_updated_at.sql"} {
		data, err := os.ReadFile("../../migrations/" + name)
		if err != nil {
			t.Fatal(err)
		}
		before[name] = &fstest.MapFile{Data: data}
	}
	if _, err := migrations.Run(ctx, conn, before, "up"); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, "INSERT INTO users(id,name,email,password_hash) VALUES ('00000000-0000-4000-8000-000000000001','Existing','existing@example.com','hash')"); err != nil {
		t.Fatal(err)
	}
	if _, err := migrations.Run(ctx, conn, os.DirFS("../../migrations"), "up"); err != nil {
		t.Fatal(err)
	}
	var role string
	if err := pool.QueryRow(ctx, "SELECT role FROM users").Scan(&role); err != nil || role != "user" {
		t.Fatal(role, err)
	}
	if _, err := pool.Exec(ctx, "UPDATE users SET role='owner'"); err == nil {
		t.Fatal("invalid role accepted")
	}
	if _, err := pool.Exec(ctx, "UPDATE users SET role='admin'"); err != nil {
		t.Fatal(err)
	}
	if _, err := migrations.Run(ctx, conn, os.DirFS("../../migrations"), "down"); err != nil {
		t.Fatal(err)
	}
	var email string
	if err := pool.QueryRow(ctx, "SELECT email FROM users").Scan(&email); err != nil || email != "existing@example.com" {
		t.Fatal(email, err)
	}
}

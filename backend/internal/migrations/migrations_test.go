package migrations_test

import (
	"os"
	"sync"
	"testing"
	"testing/fstest"

	"github.com/jackc/pgx/v5"
	"uptime-app/backend/internal/migrations"
	"uptime-app/backend/internal/testdb"
)

func TestMigrations(t *testing.T) {
	ctx, pool, url := testdb.New(t)
	conn, e := pgx.Connect(ctx, url)
	if e != nil {
		t.Fatal(e)
	}
	defer conn.Close(ctx)
	files := os.DirFS("../../migrations")
	for i, operation := range []string{"status", "up", "up", "status", "down", "up"} {
		s, e := migrations.Run(ctx, conn, files, operation)
		if e != nil {
			t.Fatal(operation, e)
		}
		want := int32(2)
		if operation == "down" {
			want = 1
		}
		if i == 0 {
			want = 0
		}
		if s.Current != want || s.Target != 2 {
			t.Fatal(operation, s)
		}
	}
	if _, e = pool.Exec(ctx, "INSERT INTO users(id,name,email,password_hash) VALUES ('00000000-0000-4000-8000-000000000001','Name','u@x.com','hash')"); e != nil {
		t.Fatal(e)
	}
	initial, e := os.ReadFile("../../migrations/00001_create_auth_tables.sql")
	if e != nil {
		t.Fatal(e)
	}
	avatar, e := os.ReadFile("../../migrations/00002_add_user_avatar.sql")
	if e != nil {
		t.Fatal(e)
	}
	extended := fstest.MapFS{"00002_add_user_avatar.sql": &fstest.MapFile{Data: avatar}, "00001_create_auth_tables.sql": &fstest.MapFile{Data: initial}, "00003_add_column.sql": &fstest.MapFile{Data: []byte("ALTER TABLE users ADD COLUMN note text;\n---- create above / drop below ----\nALTER TABLE users DROP COLUMN note;")}}
	s, e := migrations.Run(ctx, conn, extended, "up")
	if e != nil || s.Current != 3 {
		t.Fatal(s, e)
	}
	var name string
	if e = pool.QueryRow(ctx, "SELECT name FROM users").Scan(&name); e != nil || name != "Name" {
		t.Fatal(name, e)
	}
	extended["00004_failure.sql"] = &fstest.MapFile{Data: []byte("CREATE TABLE rolled_back(id int); SELECT 1/0;\n---- create above / drop below ----\nDROP TABLE rolled_back;")}
	if _, e = migrations.Run(ctx, conn, extended, "up"); e == nil {
		t.Fatal("bad migration passed")
	}
	s, e = migrations.Run(ctx, conn, extended, "status")
	if e != nil || s.Current != 3 {
		t.Fatal(s, e)
	}
	var exists bool
	if e = pool.QueryRow(ctx, "SELECT to_regclass('public.rolled_back') IS NOT NULL").Scan(&exists); e != nil || exists {
		t.Fatal("failed migration persisted", e)
	}
	delete(extended, "00004_failure.sql")
	if s, e = migrations.Run(ctx, conn, extended, "down"); e != nil || s.Current != 2 {
		t.Fatal(s, e)
	}
	if s, e = migrations.Run(ctx, conn, extended, "up"); e != nil || s.Current != 3 {
		t.Fatal(s, e)
	}
}
func TestParallelMigrations(t *testing.T) {
	ctx, pool, url := testdb.New(t)
	files := fstest.MapFS{"00001_table.sql": &fstest.MapFile{Data: []byte("SELECT pg_sleep(0.2); CREATE TABLE once_only(id int);\n---- create above / drop below ----\nDROP TABLE once_only;")}}
	var wg sync.WaitGroup
	ch := make(chan error, 2)
	for range 2 {
		wg.Go(func() {
			conn, e := pgx.Connect(ctx, url)
			if e == nil {
				defer conn.Close(ctx)
				_, e = migrations.Run(ctx, conn, files, "up")
			}
			ch <- e
		})
	}
	wg.Wait()
	close(ch)
	for e := range ch {
		if e != nil {
			t.Fatal(e)
		}
	}
	var version int
	if e := pool.QueryRow(ctx, "SELECT version FROM public.schema_version").Scan(&version); e != nil || version != 1 {
		t.Fatal(version, e)
	}
}

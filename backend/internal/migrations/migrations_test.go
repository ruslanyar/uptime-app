package migrations_test

import (
	"os"
	"sync"
	"testing"
	"testing/fstest"
	"time"

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
		want := int32(4)
		if operation == "down" {
			want = 3
		}
		if i == 0 {
			want = 0
		}
		if s.Current != want || s.Target != 4 {
			t.Fatal(operation, s)
		}
		var exists bool
		if e = pool.QueryRow(ctx, "SELECT to_regclass('public.monitors') IS NOT NULL").Scan(&exists); e != nil || exists != (want >= 3) {
			t.Fatal(operation, "monitors table", exists, e)
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
	monitors, e := os.ReadFile("../../migrations/00003_create_monitors.sql")
	if e != nil {
		t.Fatal(e)
	}
	updatedAt, e := os.ReadFile("../../migrations/00004_add_monitor_updated_at.sql")
	if e != nil {
		t.Fatal(e)
	}
	extended := fstest.MapFS{"00002_add_user_avatar.sql": &fstest.MapFile{Data: avatar}, "00001_create_auth_tables.sql": &fstest.MapFile{Data: initial}, "00003_create_monitors.sql": &fstest.MapFile{Data: monitors}, "00004_add_monitor_updated_at.sql": &fstest.MapFile{Data: updatedAt}, "00005_add_column.sql": &fstest.MapFile{Data: []byte("ALTER TABLE users ADD COLUMN note text;\n---- create above / drop below ----\nALTER TABLE users DROP COLUMN note;")}}
	s, e := migrations.Run(ctx, conn, extended, "up")
	if e != nil || s.Current != 5 {
		t.Fatal(s, e)
	}
	var name string
	if e = pool.QueryRow(ctx, "SELECT name FROM users").Scan(&name); e != nil || name != "Name" {
		t.Fatal(name, e)
	}
	extended["00006_failure.sql"] = &fstest.MapFile{Data: []byte("CREATE TABLE rolled_back(id int); SELECT 1/0;\n---- create above / drop below ----\nDROP TABLE rolled_back;")}
	if _, e = migrations.Run(ctx, conn, extended, "up"); e == nil {
		t.Fatal("bad migration passed")
	}
	s, e = migrations.Run(ctx, conn, extended, "status")
	if e != nil || s.Current != 5 {
		t.Fatal(s, e)
	}
	var exists bool
	if e = pool.QueryRow(ctx, "SELECT to_regclass('public.rolled_back') IS NOT NULL").Scan(&exists); e != nil || exists {
		t.Fatal("failed migration persisted", e)
	}
	delete(extended, "00006_failure.sql")
	if s, e = migrations.Run(ctx, conn, extended, "down"); e != nil || s.Current != 4 {
		t.Fatal(s, e)
	}
	if s, e = migrations.Run(ctx, conn, extended, "up"); e != nil || s.Current != 5 {
		t.Fatal(s, e)
	}
}
func TestParallelMigrations(t *testing.T) {
	ctx, pool, url := testdb.New(t)
	// The delay makes concurrent migrators contend for the migration lock.
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

func TestMonitorUpdatedAtMigration(t *testing.T) {
	ctx, pool, url := testdb.New(t)
	conn, err := pgx.Connect(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(ctx)
	files := os.DirFS("../../migrations")
	if _, err = migrations.Run(ctx, conn, files, "up"); err != nil {
		t.Fatal(err)
	}
	if _, err = migrations.Run(ctx, conn, files, "down"); err != nil {
		t.Fatal(err)
	}
	const owner = "00000000-0000-4000-8000-000000000001"
	const oldID = "00000000-0000-4000-8000-000000000002"
	if _, err = pool.Exec(ctx, "INSERT INTO users(id,name,email,password_hash) VALUES($1,'Name','timestamp@example.com','hash')", owner); err != nil {
		t.Fatal(err)
	}
	created := time.Date(2025, 1, 2, 3, 4, 5, 123456000, time.UTC)
	if _, err = pool.Exec(ctx, "INSERT INTO monitors(id,user_id,url,interval_seconds,created_at) VALUES($1,$2,'https://old.example',300,$3)", oldID, owner, created); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if _, err = migrations.Run(ctx, conn, files, "up"); err != nil {
			t.Fatal(err)
		}
		var actualCreated, updated time.Time
		if err = pool.QueryRow(ctx, "SELECT created_at,updated_at FROM monitors WHERE id=$1", oldID).Scan(&actualCreated, &updated); err != nil || !actualCreated.Equal(created) || !updated.Equal(created) {
			t.Fatal(actualCreated, updated, err)
		}
		if _, err = pool.Exec(ctx, "UPDATE monitors SET updated_at=NULL WHERE id=$1", oldID); err == nil {
			t.Fatal("NULL updated_at accepted")
		}
		if _, err = migrations.Run(ctx, conn, files, "down"); err != nil {
			t.Fatal(err)
		}
		var exists bool
		if err = pool.QueryRow(ctx, "SELECT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_schema='public' AND table_name='monitors' AND column_name='updated_at')").Scan(&exists); err != nil || exists {
			t.Fatal("column survived rollback", exists, err)
		}
		var actualURL string
		var interval int32
		if err = pool.QueryRow(ctx, "SELECT created_at,url,interval_seconds FROM monitors WHERE id=$1", oldID).Scan(&actualCreated, &actualURL, &interval); err != nil || !actualCreated.Equal(created) || actualURL != "https://old.example" || interval != 300 {
			t.Fatal(actualCreated, actualURL, interval, err)
		}
	}
	if _, err = migrations.Run(ctx, conn, files, "up"); err != nil {
		t.Fatal(err)
	}
	var newCreated, newUpdated time.Time
	if err = pool.QueryRow(ctx, "INSERT INTO monitors(id,user_id,url,interval_seconds) VALUES('00000000-0000-4000-8000-000000000003',$1,'https://new.example',60) RETURNING created_at,updated_at", owner).Scan(&newCreated, &newUpdated); err != nil || newCreated.IsZero() || !newCreated.Equal(newUpdated) {
		t.Fatal(newCreated, newUpdated, err)
	}
}

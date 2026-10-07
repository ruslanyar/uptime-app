package postgres_test

import (
	"errors"
	"strings"
	"sync"
	"testing"
	"uptime-app/backend/internal/auth"
	"uptime-app/backend/internal/monitor"
	"uptime-app/backend/internal/storage/postgres"
)

func TestMonitorPersistence(t *testing.T) {
	ctx, pool, a, _ := setup(t)
	u := register(t, ctx, a)
	s := monitor.NewService(postgres.New(pool))
	m, e := s.Create(ctx, u.User.ID, " https://example.com/path?q=1 ", 60)
	if e != nil || m.CreatedAt.IsZero() || !auth.ValidID(m.ID) || m.UserID != u.User.ID {
		t.Fatal(m, e)
	}
	var owner, url string
	var interval int32
	if e = pool.QueryRow(ctx, "SELECT user_id::text,url,interval_seconds FROM monitors WHERE id=$1", m.ID).Scan(&owner, &url, &interval); e != nil || owner != u.User.ID || url != m.URL || interval != 60 {
		t.Fatal(owner, url, interval, e)
	}
	if _, e = s.Create(ctx, u.User.ID, m.URL, 300); !errors.Is(e, monitor.ErrConflict) {
		t.Fatal(e)
	}
	other, e := a.Register(ctx, "Other", "other@example.com", "long password here")
	if e != nil {
		t.Fatal(e)
	}
	if _, e = s.Create(ctx, other.User.ID, m.URL, 86400); e != nil {
		t.Fatal(e)
	}
	// Exact URL spelling remains significant.
	if _, e = s.Create(ctx, u.User.ID, "https://EXAMPLE.com/path?q=1", 300); e != nil {
		t.Fatal(e)
	}
	var wg sync.WaitGroup
	results := make(chan error, 8)
	for range 8 {
		wg.Go(func() { _, e := s.Create(ctx, u.User.ID, "https://concurrent.example", 300); results <- e })
	}
	wg.Wait()
	close(results)
	success, conflicts := 0, 0
	for e := range results {
		if e == nil {
			success++
		} else if errors.Is(e, monitor.ErrConflict) {
			conflicts++
		} else {
			t.Fatal(e)
		}
	}
	if success != 1 || conflicts != 7 {
		t.Fatal(success, conflicts)
	}
	for _, tc := range []struct {
		url string
		n   int32
	}{{"", 300}, {strings.Repeat("x", 2049), 300}, {"https://db.example", 59}, {"https://db.example", 86401}} {
		if _, e = pool.Exec(ctx, "INSERT INTO monitors(id,user_id,url,interval_seconds) VALUES($1,$2,$3,$4)", auth.NewID(), u.User.ID, tc.url, tc.n); e == nil {
			t.Fatal("constraint accepted", tc)
		}
	}
	if _, e = pool.Exec(ctx, "INSERT INTO monitors(id,user_id,url,interval_seconds) VALUES($1,$2,$3,$4)", auth.NewID(), auth.NewID(), "https://missing-owner.example", 300); e == nil {
		t.Fatal("missing owner accepted")
	}
	if _, e = pool.Exec(ctx, "DELETE FROM users WHERE id=$1", u.User.ID); e != nil {
		t.Fatal(e)
	}
	var count int
	if e = pool.QueryRow(ctx, "SELECT count(*) FROM monitors WHERE user_id=$1", u.User.ID).Scan(&count); e != nil || count != 0 {
		t.Fatal(count, e)
	}
	if e = pool.QueryRow(ctx, "SELECT count(*) FROM monitors WHERE user_id=$1", other.User.ID).Scan(&count); e != nil || count != 1 {
		t.Fatal(count, e)
	}
}

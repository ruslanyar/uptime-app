package postgres_test

import (
	"context"
	"errors"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"uptime-app/backend/internal/auth"
	"uptime-app/backend/internal/migrations"
	"uptime-app/backend/internal/storage/postgres"
	"uptime-app/backend/internal/testdb"
)

func setup(t *testing.T) (context.Context, *pgxpool.Pool, *auth.Service, string) {
	t.Helper()
	ctx, pool, url := testdb.New(t)
	conn, e := pgx.Connect(ctx, url)
	if e != nil {
		t.Fatal(e)
	}
	defer conn.Close(ctx)
	if _, e = migrations.Run(ctx, conn, os.DirFS("../../../migrations"), "up"); e != nil {
		t.Fatal(e)
	}
	s, e := auth.NewService(postgres.New(pool), auth.NewTokens(strings.Repeat("s", 32), "issuer", "audience"))
	if e != nil {
		t.Fatal(e)
	}
	return ctx, pool, s, url
}
func register(t *testing.T, ctx context.Context, s *auth.Service) auth.Result {
	t.Helper()
	r, e := s.Register(ctx, "  Name  ", "  User@EXAMPLE.com  ", " long password with spaces ")
	if e != nil {
		t.Fatal(e)
	}
	return r
}
func TestLifecycle(t *testing.T) {
	ctx, pool, s, url := setup(t)
	r := register(t, ctx, s)
	if r.User.Name != "Name" || r.User.Email != "user@example.com" {
		t.Fatal(r.User)
	}
	_, e := s.Login(ctx, "unknown@example.com", " long password with spaces ")
	if !errors.Is(e, auth.ErrUnauthorized) {
		t.Fatal(e)
	}
	_, e = s.Login(ctx, r.User.Email, "wrong password here")
	if !errors.Is(e, auth.ErrUnauthorized) {
		t.Fatal(e)
	}
	_, e = s.Login(ctx, r.User.Email, "long password with spaces")
	if !errors.Is(e, auth.ErrUnauthorized) {
		t.Fatal("spaces changed", e)
	}
	independent, e := s.Login(ctx, r.User.Email, " long password with spaces ")
	if e != nil {
		t.Fatal(e)
	}
	rotated, e := s.Refresh(ctx, r.RefreshToken)
	if e != nil {
		t.Fatal(e)
	}
	if rotated.RefreshToken == r.RefreshToken || !rotated.SessionExpires.Equal(r.SessionExpires.Truncate(time.Microsecond)) {
		t.Fatal("rotation extended expiry")
	}
	_, e = s.Refresh(ctx, r.RefreshToken)
	if !errors.Is(e, auth.ErrUnauthorized) {
		t.Fatal("reuse accepted", e)
	}
	_, e = s.Refresh(ctx, rotated.RefreshToken)
	if !errors.Is(e, auth.ErrUnauthorized) {
		t.Fatal("revocation did not persist", e)
	}
	if _, e = s.Refresh(ctx, independent.RefreshToken); e != nil {
		t.Fatal("independent session revoked", e)
	}
	if u, e := s.Me(ctx, r.AccessToken); e != nil || u.ID != r.User.ID {
		t.Fatal("JWT revoked prematurely", e)
	}
	// Recreate pool and service to prove no state depends on process memory.
	other, e := pgxpool.New(ctx, url)
	if e != nil {
		t.Fatal(e)
	}
	defer other.Close()
	restarted, e := auth.NewService(postgres.New(other), auth.NewTokens(strings.Repeat("s", 32), "issuer", "audience"))
	if e != nil {
		t.Fatal(e)
	}
	if _, e = restarted.Login(ctx, r.User.Email, " long password with spaces "); e != nil {
		t.Fatal(e)
	}
	if e = s.Logout(ctx, independent.RefreshToken); e != nil {
		t.Fatal(e)
	}
	if e = s.Logout(ctx, independent.RefreshToken); e != nil {
		t.Fatal(e)
	}
	if e = s.Logout(ctx, ""); e != nil {
		t.Fatal(e)
	}
	var count int
	if e = pool.QueryRow(ctx, "SELECT count(*) FROM refresh_tokens WHERE used_at IS NOT NULL").Scan(&count); e != nil || count < 2 {
		t.Fatal("used tokens not retained", count, e)
	}
}
func TestConcurrentRegistration(t *testing.T) {
	ctx, pool, s, _ := setup(t)
	var wg sync.WaitGroup
	results := make(chan error, 2)
	for range 2 {
		wg.Go(func() { _, e := s.Register(ctx, "Name", "same@example.com", "long password for test"); results <- e })
	}
	wg.Wait()
	close(results)
	success, conflicts := 0, 0
	for e := range results {
		if e == nil {
			success++
		} else if errors.Is(e, auth.ErrConflict) {
			conflicts++
		} else {
			t.Fatal(e)
		}
	}
	if success != 1 || conflicts != 1 {
		t.Fatal(success, conflicts)
	}
	var n int
	_ = pool.QueryRow(ctx, "SELECT count(*) FROM sessions").Scan(&n)
	if n != 1 {
		t.Fatal("orphan sessions", n)
	}
}
func TestConcurrentRefresh(t *testing.T) {
	ctx, _, s, _ := setup(t)
	r := register(t, ctx, s)
	type result struct {
		r auth.Result
		e error
	}
	ch := make(chan result, 2)
	var wg sync.WaitGroup
	for range 2 {
		wg.Go(func() { r, e := s.Refresh(ctx, r.RefreshToken); ch <- result{r, e} })
	}
	wg.Wait()
	close(ch)
	success, denied := 0, 0
	var latest string
	for res := range ch {
		if res.e == nil {
			success++
			latest = res.r.RefreshToken
		} else if errors.Is(res.e, auth.ErrUnauthorized) {
			denied++
		} else {
			t.Fatal(res.e)
		}
	}
	if success != 1 || denied != 1 {
		t.Fatal(success, denied)
	}
	if _, e := s.Refresh(ctx, latest); !errors.Is(e, auth.ErrUnauthorized) {
		t.Fatal("new token valid after reuse", e)
	}
}
func TestExpiryAndLogout(t *testing.T) {
	ctx, pool, s, _ := setup(t)
	r := register(t, ctx, s)
	if _, e := pool.Exec(ctx, "UPDATE sessions SET expires_at=now()-interval '1 second'"); e != nil {
		t.Fatal(e)
	}
	if _, e := s.Refresh(ctx, r.RefreshToken); !errors.Is(e, auth.ErrUnauthorized) {
		t.Fatal(e)
	}
	login, e := s.Login(ctx, r.User.Email, " long password with spaces ")
	if e != nil {
		t.Fatal(e)
	}
	if e = s.Logout(ctx, login.RefreshToken); e != nil {
		t.Fatal(e)
	}
	if _, e = s.Refresh(ctx, login.RefreshToken); !errors.Is(e, auth.ErrUnauthorized) {
		t.Fatal(e)
	}
}
func TestAtomicity(t *testing.T) {
	ctx, pool, s, _ := setup(t)
	// Deferred constraint trigger fails at commit, after all queries have succeeded.
	_, e := pool.Exec(ctx, `CREATE FUNCTION fail_commit() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'forced commit failure'; END $$; CREATE CONSTRAINT TRIGGER reject_token AFTER INSERT ON refresh_tokens DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION fail_commit();`)
	if e != nil {
		t.Fatal(e)
	}
	r, e := s.Register(ctx, "Name", "u@example.com", "long password here")
	if e == nil || r.RefreshToken != "" {
		t.Fatal("success on failed commit", e)
	}
	var n int
	if e = pool.QueryRow(ctx, "SELECT count(*) FROM users").Scan(&n); e != nil || n != 0 {
		t.Fatal("registration leaked", n, e)
	}
	if _, e = pool.Exec(ctx, "DROP TRIGGER reject_token ON refresh_tokens"); e != nil {
		t.Fatal(e)
	}
	r = register(t, ctx, s)
	if _, e = pool.Exec(ctx, `CREATE CONSTRAINT TRIGGER reject_token AFTER INSERT ON refresh_tokens DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION fail_commit()`); e != nil {
		t.Fatal(e)
	}
	if _, e = s.Refresh(ctx, r.RefreshToken); e == nil {
		t.Fatal("rotation commit accepted")
	}
	if _, e = pool.Exec(ctx, "DROP TRIGGER reject_token ON refresh_tokens"); e != nil {
		t.Fatal(e)
	}
	if _, e = s.Refresh(ctx, r.RefreshToken); e != nil {
		t.Fatal("rotation failed to rollback", e)
	}
}

func TestExpiryWhileWaitingForLock(t *testing.T) {
	ctx, pool, s, _ := setup(t)
	r := register(t, ctx, s)
	if _, e := pool.Exec(ctx, "UPDATE sessions SET expires_at=now()+interval '0.2 seconds'"); e != nil {
		t.Fatal(e)
	}
	tx, e := pool.Begin(ctx)
	if e != nil {
		t.Fatal(e)
	}
	defer tx.Rollback(ctx)
	if _, e = tx.Exec(ctx, "SELECT id FROM sessions FOR UPDATE"); e != nil {
		t.Fatal(e)
	}
	done := make(chan error, 1)
	go func() { _, e := s.Refresh(ctx, r.RefreshToken); done <- e }()
	// Keep the session locked beyond expiry to check validation after lock acquisition.
	if _, e = tx.Exec(ctx, "SELECT pg_sleep(0.3)"); e != nil {
		t.Fatal(e)
	}
	if e = tx.Commit(ctx); e != nil {
		t.Fatal(e)
	}
	if e = <-done; !errors.Is(e, auth.ErrUnauthorized) {
		t.Fatal("expired session rotated after waiting for lock", e)
	}
}

func TestUpdateProfile(t *testing.T) {
	ctx, pool, s, _ := setup(t)
	r := register(t, ctx, s)
	other, e := s.Register(ctx, "Other", "other@example.com", "long password here")
	if e != nil {
		t.Fatal(e)
	}
	u, e := s.UpdateProfile(ctx, r.AccessToken, "  Новое имя  ")
	if e != nil || u.ID != r.User.ID || u.Email != r.User.Email || u.Name != "Новое имя" {
		t.Fatal(u, e)
	}
	persisted, e := postgres.New(pool).ByID(ctx, r.User.ID)
	if e != nil || persisted != u {
		t.Fatal(persisted, e)
	}
	unchanged, e := s.Me(ctx, other.AccessToken)
	if e != nil || unchanged != other.User {
		t.Fatal(unchanged, e)
	}
	if _, e := s.Login(ctx, r.User.Email, " long password with spaces "); e != nil {
		t.Fatal("password changed", e)
	}
}

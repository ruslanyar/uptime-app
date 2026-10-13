package postgres_test

import (
	"context"
	"sync"
	"testing"
	"time"
	"uptime-app/backend/internal/monitor"
	"uptime-app/backend/internal/storage/postgres"
)

func successfulCheck() monitor.CheckResult {
	status := int32(200)
	ms := int64(123)
	return monitor.CheckResult{CheckedAt: time.Now().UTC(), HTTPStatus: &status, ResponseTimeMS: &ms}
}

func TestCheckPersistenceAndSchedule(t *testing.T) {
	ctx, pool, a, _ := setup(t)
	u := register(t, ctx, a)
	store := postgres.New(pool)
	service := monitor.NewService(store)
	m, err := service.Create(ctx, u.User.ID, "https://example.com", 60)
	if err != nil || m.Monitoring.Status != "unknown" || m.Monitoring.LastCheck != nil || m.Monitoring.NextCheckAt.IsZero() {
		t.Fatal(m, err)
	}
	jobs, err := store.ClaimMonitorChecks(ctx, 10)
	if err != nil || len(jobs) != 1 {
		t.Fatal(jobs, err)
	}
	if more, err := store.ClaimMonitorChecks(ctx, 10); err != nil || len(more) != 0 {
		t.Fatal(more, err)
	}
	before := time.Now()
	if ok, err := store.CompleteMonitorCheck(ctx, jobs[0], "up", successfulCheck()); err != nil || !ok {
		t.Fatal(ok, err)
	}
	items, err := service.List(ctx, u.User.ID)
	if err != nil || len(items) != 1 {
		t.Fatal(items, err)
	}
	checked := items[0]
	if checked.Monitoring.Status != "up" || checked.Monitoring.LastCheck == nil || *checked.Monitoring.LastCheck.HTTPStatus != 200 || *checked.Monitoring.LastCheck.ResponseTimeMS != 123 || checked.Monitoring.IsStale || !checked.UpdatedAt.Equal(m.UpdatedAt) || checked.Monitoring.NextCheckAt.Before(before.Add(60*time.Second)) {
		t.Fatal(checked)
	}
	if ok, err := store.CompleteMonitorCheck(ctx, jobs[0], "up", successfulCheck()); err != nil || ok {
		t.Fatal(ok, err)
	}
	if _, err := pool.Exec(ctx, "UPDATE monitors SET next_check_at=now()-interval '31 seconds' WHERE id=$1", m.ID); err != nil {
		t.Fatal(err)
	}
	items, err = service.List(ctx, u.User.ID)
	if err != nil || !items[0].Monitoring.IsStale || items[0].Monitoring.Status != "up" {
		t.Fatal(items, err)
	}
	// Restart is represented by a new Store; overdue work is claimed once, not replayed.
	jobs, err = postgres.New(pool).ClaimMonitorChecks(ctx, 10)
	if err != nil || len(jobs) != 1 {
		t.Fatal(jobs, err)
	}
	code := "timeout"
	if ok, err := store.CompleteMonitorCheck(ctx, jobs[0], "down", monitor.CheckResult{CheckedAt: time.Now().UTC(), ErrorCode: &code}); err != nil || !ok {
		t.Fatal(ok, err)
	}
	items, err = service.List(ctx, u.User.ID)
	if err != nil || items[0].Monitoring.Status != "down" || items[0].Monitoring.LastCheck.HTTPStatus != nil {
		t.Fatal(items, err)
	}
}

func TestCheckLeaseInvalidation(t *testing.T) {
	ctx, pool, a, _ := setup(t)
	u := register(t, ctx, a)
	store := postgres.New(pool)
	service := monitor.NewService(store)
	m, err := service.Create(ctx, u.User.ID, "https://example.com", 60)
	if err != nil {
		t.Fatal(err)
	}
	claim := func() monitor.CheckJob {
		t.Helper()
		jobs, err := store.ClaimMonitorChecks(ctx, 1)
		if err != nil || len(jobs) != 1 {
			t.Fatal(jobs, err)
		}
		return jobs[0]
	}
	reject := func(job monitor.CheckJob) {
		t.Helper()
		ok, err := store.CompleteMonitorCheck(ctx, job, "up", successfulCheck())
		if err != nil || ok {
			t.Fatal(ok, err)
		}
	}
	first := claim()
	if _, err := pool.Exec(ctx, "UPDATE monitors SET lease_until=now()-interval '1 second' WHERE id=$1", m.ID); err != nil {
		t.Fatal(err)
	}
	reject(first)
	replacement := claim()
	if replacement.LeaseToken == first.LeaseToken {
		t.Fatal("reused lease token")
	}
	reject(first)
	if ok, err := store.CompleteMonitorCheck(ctx, replacement, "up", successfulCheck()); err != nil || !ok {
		t.Fatal(ok, err)
	}
	m, err = service.Update(ctx, u.User.ID, m.ID, m.URL, 300)
	if err != nil || m.Monitoring.Status != "up" {
		t.Fatal(m, err)
	}
	old := claim()
	m, err = service.Update(ctx, u.User.ID, m.ID, "https://changed.example.com", 300)
	if err != nil || m.Monitoring.Status != "unknown" || m.Monitoring.LastCheck != nil {
		t.Fatal(m, err)
	}
	reject(old)
	old = claim()
	if _, err = service.Update(ctx, u.User.ID, m.ID, m.URL, 300); err != nil {
		t.Fatal(err)
	}
	reject(old)
	current := claim()
	if err = service.Delete(ctx, u.User.ID, m.ID); err != nil {
		t.Fatal(err)
	}
	reject(current)
}

func TestConcurrentCheckClaims(t *testing.T) {
	ctx, pool, a, _ := setup(t)
	u := register(t, ctx, a)
	store := postgres.New(pool)
	for _, url := range []string{"https://a.example.com", "https://b.example.com", "https://c.example.com", "https://d.example.com"} {
		if _, err := monitor.NewService(store).Create(ctx, u.User.ID, url, 60); err != nil {
			t.Fatal(err)
		}
	}
	var wg sync.WaitGroup
	results := make(chan []monitor.CheckJob, 8)
	errs := make(chan error, 8)
	for range 8 {
		wg.Go(func() { jobs, err := store.ClaimMonitorChecks(ctx, 2); results <- jobs; errs <- err })
	}
	wg.Wait()
	close(results)
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	seen := map[string]bool{}
	for jobs := range results {
		if len(jobs) > 2 {
			t.Fatal(jobs)
		}
		for _, job := range jobs {
			if seen[job.ID] {
				t.Fatal("duplicate claim", job.ID)
			}
			seen[job.ID] = true
		}
	}
	if len(seen) != 4 {
		t.Fatal(seen)
	}
}

func TestBlockedCheckAndOwnership(t *testing.T) {
	ctx, pool, a, _ := setup(t)
	u := register(t, ctx, a)
	store := postgres.New(pool)
	service := monitor.NewService(store)
	m, err := service.Create(ctx, u.User.ID, "http://127.0.0.1", 60)
	if err != nil {
		t.Fatal(err)
	}
	jobs, err := store.ClaimMonitorChecks(ctx, 1)
	if err != nil || len(jobs) != 1 {
		t.Fatal(jobs, err)
	}
	checker := monitor.NewChecker()
	defer checker.Close()
	status, result := checker.Check(context.Background(), m.URL)
	if ok, err := store.CompleteMonitorCheck(ctx, jobs[0], status, result); err != nil || !ok {
		t.Fatal(ok, err)
	}
	other, err := a.Register(ctx, "Other", "other@example.com", "long password here")
	if err != nil {
		t.Fatal(err)
	}
	if items, err := service.List(ctx, other.User.ID); err != nil || len(items) != 0 {
		t.Fatal(items, err)
	}
	items, err := service.List(ctx, u.User.ID)
	if err != nil || items[0].Monitoring.Status != "blocked" || *items[0].Monitoring.LastCheck.ErrorCode != "blocked_address" {
		t.Fatal(items, err)
	}
}

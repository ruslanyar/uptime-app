package monitor

import (
	"context"
	"errors"
	"sync"
	"testing"
	"testing/synctest"
	"time"
)

type workerStore struct {
	mu           sync.Mutex
	available    int
	claims       []int32
	completed    int
	failClaim    bool
	failComplete bool
}

func (s *workerStore) ClaimMonitorChecks(ctx context.Context, limit int32) ([]CheckJob, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.claims = append(s.claims, limit)
	if s.failClaim {
		return nil, errors.New("database unavailable")
	}
	jobs := []CheckJob{}
	for s.available > 0 && int32(len(jobs)) < limit {
		s.available--
		jobs = append(jobs, CheckJob{ID: "id", URL: "https://example.com", LeaseToken: "token"})
	}
	return jobs, nil
}
func (s *workerStore) CompleteMonitorCheck(context.Context, CheckJob, string, CheckResult) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.completed++
	if s.failComplete {
		return false, errors.New("write unavailable")
	}
	return true, nil
}

type checkerFunc func(context.Context, string) (string, CheckResult)

func (f checkerFunc) Check(ctx context.Context, url string) (string, CheckResult) { return f(ctx, url) }

func TestWorkerConcurrencyAndShutdown(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		store := &workerStore{available: 25}
		var mu sync.Mutex
		active, peak, finished := 0, 0, 0
		checker := checkerFunc(func(ctx context.Context, _ string) (string, CheckResult) {
			mu.Lock()
			active++
			if active > peak {
				peak = active
			}
			mu.Unlock()
			<-ctx.Done()
			mu.Lock()
			active--
			finished++
			mu.Unlock()
			return "down", CheckResult{}
		})
		done := make(chan struct{})
		go func() { RunWorker(ctx, store, checker); close(done) }()
		synctest.Wait()
		time.Sleep(3 * time.Second)
		synctest.Wait()
		mu.Lock()
		if peak != 10 || active != 10 {
			t.Fatal(peak, active)
		}
		mu.Unlock()
		cancel()
		synctest.Wait()
		<-done
		if active != 0 || finished != 10 || store.completed != 0 || store.available != 15 {
			t.Fatal(active, finished, store.completed, store.available)
		}
	})
}

func TestWorkerSchedulesOnlyAvailableSlots(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		store := &workerStore{available: 25}
		checker := checkerFunc(func(ctx context.Context, _ string) (string, CheckResult) {
			select {
			case <-ctx.Done():
			case <-time.After(2 * time.Second):
			}
			return "up", CheckResult{CheckedAt: time.Now()}
		})
		done := make(chan struct{})
		go func() { RunWorker(ctx, store, checker); close(done) }()
		synctest.Wait()
		time.Sleep(10 * time.Second)
		synctest.Wait()
		cancel()
		synctest.Wait()
		<-done
		if store.available != 0 || store.completed != 25 {
			t.Fatal(store.available, store.completed)
		}
		for _, limit := range store.claims {
			if limit < 1 || limit > 10 {
				t.Fatal(limit)
			}
		}
	})
}

func TestWorkerStorageBackoff(t *testing.T) {
	for _, failComplete := range []bool{false, true} {
		synctest.Test(t, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			store := &workerStore{available: 1, failClaim: !failComplete, failComplete: failComplete}
			checker := checkerFunc(func(context.Context, string) (string, CheckResult) { return "up", CheckResult{CheckedAt: time.Now()} })
			done := make(chan struct{})
			go func() { RunWorker(ctx, store, checker); close(done) }()
			synctest.Wait()
			time.Sleep(4 * time.Second)
			synctest.Wait()
			store.mu.Lock()
			if len(store.claims) != 1 {
				t.Fatal(store.claims)
			}
			store.mu.Unlock()
			time.Sleep(time.Second)
			synctest.Wait()
			store.mu.Lock()
			if len(store.claims) != 2 {
				t.Fatal(store.claims)
			}
			store.mu.Unlock()
			cancel()
			synctest.Wait()
			<-done
		})
	}
}

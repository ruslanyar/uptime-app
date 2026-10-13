package monitor

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

const workerConcurrency = 10

type CheckJob struct{ ID, URL, LeaseToken string }
type CheckStore interface {
	ClaimMonitorChecks(context.Context, int32) ([]CheckJob, error)
	CompleteMonitorCheck(context.Context, CheckJob, string, CheckResult) (bool, error)
}
type SiteChecker interface {
	Check(context.Context, string) (string, CheckResult)
}

// RunWorker polls persisted schedules until cancellation and then joins all work.
// Store operations have a five-second budget; storage failures pause dispatch for five seconds.
// A cancelled check or failed write leaves its lease to expire rather than recording downtime.
func RunWorker(ctx context.Context, store CheckStore, checker SiteChecker) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	var wg sync.WaitGroup
	defer wg.Wait()
	completed := make(chan error, workerConcurrency)
	active := 0
	var retryAt time.Time
	dispatch := func() {
		if active == workerConcurrency || time.Now().Before(retryAt) || ctx.Err() != nil {
			return
		}
		op, cancel := context.WithTimeout(ctx, 5*time.Second)
		jobs, err := store.ClaimMonitorChecks(op, int32(workerConcurrency-active))
		cancel()
		if err != nil {
			if ctx.Err() == nil {
				slog.Error("monitor scheduler storage failure", "operation", "claim", "error", err)
				retryAt = time.Now().Add(5 * time.Second)
			}
			return
		}
		for _, job := range jobs {
			active++
			wg.Go(func() {
				status, result := checker.Check(ctx, job.URL)
				var err error
				if ctx.Err() == nil {
					op, cancel := context.WithTimeout(ctx, 5*time.Second)
					_, err = store.CompleteMonitorCheck(op, job, status, result)
					cancel()
				}
				completed <- err
			})
		}
	}
	dispatch()
	for {
		select {
		case <-ctx.Done():
			return
		case err := <-completed:
			active--
			if err != nil && ctx.Err() == nil {
				slog.Error("monitor scheduler storage failure", "operation", "complete", "error", err)
				retryAt = time.Now().Add(5 * time.Second)
			}
		case <-ticker.C:
			dispatch()
		}
	}
}

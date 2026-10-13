package postgres

import (
	"context"
	"errors"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"time"
	"uptime-app/backend/internal/monitor"
	"uptime-app/backend/internal/storage/postgres/sqlc"
)

func (s *Store) CreateMonitor(ctx context.Context, m monitor.Monitor) (monitor.Monitor, error) {
	row, e := s.q.CreateMonitor(ctx, sqlc.CreateMonitorParams{ID: uuid(m.ID), UserID: uuid(m.UserID), Url: m.URL, IntervalSeconds: m.IntervalSeconds})
	if e != nil {
		var p *pgconn.PgError
		// The constraint also prevents duplicate URLs during concurrent requests.
		if errors.As(e, &p) && p.Code == "23505" && p.ConstraintName == "monitors_user_url_key" {
			return monitor.Monitor{}, monitor.ErrConflict
		}
		return monitor.Monitor{}, e
	}
	return monitorRow(row), nil
}

func (s *Store) ListMonitors(ctx context.Context, userID string) ([]monitor.Monitor, error) {
	rows, e := s.q.ListMonitors(ctx, uuid(userID))
	if e != nil {
		return nil, e
	}
	result := make([]monitor.Monitor, 0, len(rows))
	for _, row := range rows {
		result = append(result, monitorRow(row))
	}
	return result, nil
}

// UpdateMonitor returns ErrNotFound for both missing rows and another user's rows.
func (s *Store) UpdateMonitor(ctx context.Context, m monitor.Monitor) (monitor.Monitor, error) {
	row, err := s.q.UpdateMonitor(ctx, sqlc.UpdateMonitorParams{ID: uuid(m.ID), UserID: uuid(m.UserID), Url: m.URL, IntervalSeconds: m.IntervalSeconds})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return monitor.Monitor{}, monitor.ErrNotFound
		}
		var p *pgconn.PgError
		if errors.As(err, &p) && p.Code == "23505" && p.ConstraintName == "monitors_user_url_key" {
			return monitor.Monitor{}, monitor.ErrConflict
		}
		return monitor.Monitor{}, err
	}
	return monitorRow(row), nil
}

// DeleteMonitor returns ErrNotFound for missing or foreign rows, including repeat deletes.
func (s *Store) DeleteMonitor(ctx context.Context, userID, monitorID string) error {
	count, err := s.q.DeleteMonitor(ctx, sqlc.DeleteMonitorParams{ID: uuid(monitorID), UserID: uuid(userID)})
	if err != nil {
		return err
	}
	if count == 0 {
		return monitor.ErrNotFound
	}
	return nil
}

func monitorRow(row sqlc.Monitor) monitor.Monitor {
	state := monitor.Monitoring{Status: row.CheckStatus, NextCheckAt: row.NextCheckAt.Time.UTC()}
	if row.CheckedAt.Valid {
		result := &monitor.CheckResult{CheckedAt: row.CheckedAt.Time.UTC()}
		if row.HttpStatus.Valid {
			result.HTTPStatus = &row.HttpStatus.Int32
		}
		if row.ResponseTimeMs.Valid {
			result.ResponseTimeMS = &row.ResponseTimeMs.Int64
		}
		if row.CheckError.Valid {
			result.ErrorCode = &row.CheckError.String
		}
		state.LastCheck = result
		state.IsStale = time.Now().After(state.NextCheckAt.Add(30 * time.Second))
	}
	return monitor.Monitor{ID: id(row.ID), UserID: id(row.UserID), URL: row.Url, IntervalSeconds: row.IntervalSeconds, CreatedAt: row.CreatedAt.Time.UTC(), UpdatedAt: row.UpdatedAt.Time.UTC(), Monitoring: state}
}

// ClaimMonitorChecks atomically leases at most limit due rows for thirty seconds.
// The returned tokens must be supplied when completing work; no transaction stays open.
func (s *Store) ClaimMonitorChecks(ctx context.Context, limit int32) ([]monitor.CheckJob, error) {
	rows, err := s.q.ClaimMonitorChecks(ctx, limit)
	if err != nil {
		return nil, err
	}
	jobs := make([]monitor.CheckJob, 0, len(rows))
	for _, row := range rows {
		jobs = append(jobs, monitor.CheckJob{ID: id(row.ID), URL: row.Url, LeaseToken: id(row.LeaseToken)})
	}
	return jobs, nil
}

// CompleteMonitorCheck ignores deleted, edited, expired or replaced leases.
// It schedules from database completion time and never changes configuration timestamps.
func (s *Store) CompleteMonitorCheck(ctx context.Context, job monitor.CheckJob, status string, result monitor.CheckResult) (bool, error) {
	args := sqlc.CompleteMonitorCheckParams{ID: uuid(job.ID), LeaseToken: uuid(job.LeaseToken), CheckStatus: status, CheckedAt: pgtype.Timestamptz{Time: result.CheckedAt, Valid: true}}
	if result.HTTPStatus != nil {
		args.HttpStatus = pgtype.Int4{Int32: *result.HTTPStatus, Valid: true}
	}
	if result.ResponseTimeMS != nil {
		args.ResponseTimeMs = pgtype.Int8{Int64: *result.ResponseTimeMS, Valid: true}
	}
	if result.ErrorCode != nil {
		args.CheckError = pgtype.Text{String: *result.ErrorCode, Valid: true}
	}
	count, err := s.q.CompleteMonitorCheck(ctx, args)
	return count == 1, err
}

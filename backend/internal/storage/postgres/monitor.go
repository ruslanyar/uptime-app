package postgres

import (
	"context"
	"errors"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
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
	return monitor.Monitor{ID: id(row.ID), UserID: id(row.UserID), URL: row.Url, IntervalSeconds: row.IntervalSeconds, CreatedAt: row.CreatedAt.Time.UTC(), UpdatedAt: row.UpdatedAt.Time.UTC()}, nil
}

func (s *Store) ListMonitors(ctx context.Context, userID string) ([]monitor.Monitor, error) {
	rows, e := s.q.ListMonitors(ctx, uuid(userID))
	if e != nil {
		return nil, e
	}
	result := make([]monitor.Monitor, 0, len(rows))
	for _, row := range rows {
		result = append(result, monitor.Monitor{ID: id(row.ID), UserID: id(row.UserID), URL: row.Url, IntervalSeconds: row.IntervalSeconds, CreatedAt: row.CreatedAt.Time.UTC(), UpdatedAt: row.UpdatedAt.Time.UTC()})
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
	return monitor.Monitor{ID: id(row.ID), UserID: id(row.UserID), URL: row.Url, IntervalSeconds: row.IntervalSeconds, CreatedAt: row.CreatedAt.Time.UTC(), UpdatedAt: row.UpdatedAt.Time.UTC()}, nil
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

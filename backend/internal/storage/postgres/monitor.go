package postgres

import (
	"context"
	"errors"
	"github.com/jackc/pgx/v5/pgconn"
	"uptime-app/backend/internal/monitor"
	"uptime-app/backend/internal/storage/postgres/sqlc"
)

func (s *Store) CreateMonitor(ctx context.Context, m monitor.Monitor) (monitor.Monitor, error) {
	row, e := s.q.CreateMonitor(ctx, sqlc.CreateMonitorParams{ID: uuid(m.ID), UserID: uuid(m.UserID), Url: m.URL, IntervalSeconds: m.IntervalSeconds})
	if e != nil {
		var p *pgconn.PgError
		if errors.As(e, &p) && p.Code == "23505" && p.ConstraintName == "monitors_user_url_key" {
			return monitor.Monitor{}, monitor.ErrConflict
		}
		return monitor.Monitor{}, e
	}
	return monitor.Monitor{ID: id(row.ID), UserID: id(row.UserID), URL: row.Url, IntervalSeconds: row.IntervalSeconds, CreatedAt: row.CreatedAt.Time.UTC()}, nil
}

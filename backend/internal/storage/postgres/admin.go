package postgres

import (
	"context"
	"errors"
	"github.com/jackc/pgx/v5"
	"uptime-app/backend/internal/admin"
	"uptime-app/backend/internal/auth"
	"uptime-app/backend/internal/storage/postgres/sqlc"
)

// ListUsers uses one snapshot so the total and page agree under concurrent deletions.
func (s *Store) ListUsers(ctx context.Context, page int32) (admin.List, error) {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return admin.List{}, err
	}
	defer rollback(tx)
	q := s.q.WithTx(tx)
	total, err := q.CountUsers(ctx)
	if err != nil {
		return admin.List{}, err
	}
	rows, err := q.ListUsers(ctx, sqlc.ListUsersParams{Limit: admin.PageSize, Offset: int64(page-1) * admin.PageSize})
	if err != nil {
		return admin.List{}, err
	}
	result := admin.List{Users: make([]admin.User, 0, len(rows)), Total: total, Page: page, PageSize: admin.PageSize}
	for _, row := range rows {
		result.Users = append(result.Users, admin.User{ID: id(row.ID), Name: row.Name, Email: row.Email, Role: row.Role})
	}
	return result, tx.Commit(ctx)
}

// DeleteUser locks both accounts in UUID order to serialize role changes without deadlocking opposing deletes.
func (s *Store) DeleteUser(ctx context.Context, actor, target string) (string, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return "", err
	}
	defer rollback(tx)
	q := s.q.WithTx(tx)
	first, second := actor, target
	if first > second {
		first, second = second, first
	}
	accounts := make(map[string]sqlc.User, 2)
	for _, key := range []string{first, second} {
		row, err := q.LockUser(ctx, uuid(key))
		if errors.Is(err, pgx.ErrNoRows) {
			if key == actor {
				return "", auth.ErrUnauthorized
			}
			return "", admin.ErrNotFound
		}
		if err != nil {
			return "", err
		}
		accounts[key] = row
	}
	if accounts[actor].Role != "admin" {
		return "", admin.ErrForbidden
	}
	if actor == target || accounts[target].Role == "admin" {
		return "", admin.ErrProtected
	}
	if err := q.DeleteUser(ctx, uuid(target)); err != nil {
		return "", err
	}
	if err := tx.Commit(ctx); err != nil {
		return "", err
	}
	return accounts[target].AvatarUrl, nil
}

// GrantAdmin shares PostgreSQL's row lock with DeleteUser; missing users are never created.
func (s *Store) GrantAdmin(ctx context.Context, email string) error {
	count, err := s.q.GrantAdmin(ctx, email)
	if err != nil {
		return err
	}
	if count == 0 {
		return admin.ErrNotFound
	}
	return nil
}

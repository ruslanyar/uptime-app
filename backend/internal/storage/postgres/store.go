package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"uptime-app/backend/internal/auth"
	"uptime-app/backend/internal/storage/postgres/sqlc"
)

type Store struct {
	pool *pgxpool.Pool
	q    *sqlc.Queries
}

func New(pool *pgxpool.Pool) *Store { return &Store{pool, sqlc.New(pool)} }
func uuid(id string) pgtype.UUID    { var u pgtype.UUID; _ = u.Scan(id); return u }
func id(u pgtype.UUID) string       { v, _ := u.Value(); return v.(string) }
func user(u sqlc.User) auth.User    { return auth.User{ID: id(u.ID), Name: u.Name, Email: u.Email} }
func storageError(e error) error {
	if errors.Is(e, pgx.ErrNoRows) {
		return auth.ErrUnauthorized
	}
	var p *pgconn.PgError
	if errors.As(e, &p) && p.Code == "23505" && p.ConstraintName == "users_email_key" {
		return auth.ErrConflict
	}
	return e
}
func rollback(tx pgx.Tx) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = tx.Rollback(ctx)
}
func createSession(ctx context.Context, q *sqlc.Queries, s auth.Session, hash []byte) error {
	if e := q.CreateSession(ctx, sqlc.CreateSessionParams{ID: uuid(s.ID), UserID: uuid(s.UserID), ExpiresAt: pgtype.Timestamptz{Time: s.ExpiresAt, Valid: true}}); e != nil {
		return e
	}
	return q.CreateToken(ctx, sqlc.CreateTokenParams{Hash: hash, SessionID: uuid(s.ID)})
}
func (s *Store) Register(ctx context.Context, a auth.Account, session auth.Session, hash []byte) error {
	tx, e := s.pool.Begin(ctx)
	if e != nil {
		return e
	}
	defer rollback(tx)
	q := s.q.WithTx(tx)
	_, e = q.CreateUser(ctx, sqlc.CreateUserParams{ID: uuid(a.ID), Name: a.Name, Email: a.Email, PasswordHash: a.PasswordHash})
	if e != nil {
		return storageError(e)
	}
	if e = createSession(ctx, q, session, hash); e != nil {
		return e
	}
	return tx.Commit(ctx)
}
func (s *Store) ByEmail(ctx context.Context, email string) (auth.Account, error) {
	u, e := s.q.UserByEmail(ctx, email)
	if e != nil {
		return auth.Account{}, storageError(e)
	}
	return auth.Account{User: user(u), PasswordHash: u.PasswordHash}, nil
}
func (s *Store) ByID(ctx context.Context, userID string) (auth.User, error) {
	u, e := s.q.UserByID(ctx, uuid(userID))
	if e != nil {
		return auth.User{}, storageError(e)
	}
	return user(u), nil
}
func (s *Store) CreateSession(ctx context.Context, session auth.Session, hash []byte) error {
	tx, e := s.pool.Begin(ctx)
	if e != nil {
		return e
	}
	defer rollback(tx)
	if e = createSession(ctx, s.q.WithTx(tx), session, hash); e != nil {
		return e
	}
	return tx.Commit(ctx)
}
func (s *Store) Rotate(ctx context.Context, old, newHash []byte) (auth.User, auth.Session, error) {
	tx, e := s.pool.Begin(ctx)
	if e != nil {
		return auth.User{}, auth.Session{}, e
	}
	defer rollback(tx)
	q := s.q.WithTx(tx)
	token, e := q.TokenByHash(ctx, old)
	if e != nil {
		return auth.User{}, auth.Session{}, storageError(e)
	}
	session, e := q.LockSession(ctx, token.SessionID)
	if e != nil {
		return auth.User{}, auth.Session{}, storageError(e)
	}
	// Read again after acquiring the session lock: another rotation may have used it.
	token, e = q.TokenByHash(ctx, old)
	if e != nil {
		return auth.User{}, auth.Session{}, storageError(e)
	}
	// A competing transaction may have held the lock until the session expired.
	now := time.Now()
	if session.Revoked || !session.ExpiresAt.Time.After(now) {
		return auth.User{}, auth.Session{}, auth.ErrUnauthorized
	}
	if token.UsedAt.Valid {
		if e = q.RevokeSession(ctx, session.ID); e != nil {
			return auth.User{}, auth.Session{}, e
		}
		if e = tx.Commit(ctx); e != nil {
			return auth.User{}, auth.Session{}, e
		}
		return auth.User{}, auth.Session{}, auth.ErrUnauthorized
	}
	if e = q.UseToken(ctx, sqlc.UseTokenParams{Hash: old, UsedAt: pgtype.Timestamptz{Time: now, Valid: true}}); e != nil {
		return auth.User{}, auth.Session{}, e
	}
	if e = q.CreateToken(ctx, sqlc.CreateTokenParams{Hash: newHash, SessionID: session.ID}); e != nil {
		return auth.User{}, auth.Session{}, e
	}
	u, e := q.UserByID(ctx, session.UserID)
	if e != nil {
		return auth.User{}, auth.Session{}, storageError(e)
	}
	if e = tx.Commit(ctx); e != nil {
		return auth.User{}, auth.Session{}, e
	}
	return user(u), auth.Session{ID: id(session.ID), UserID: id(session.UserID), ExpiresAt: session.ExpiresAt.Time}, nil
}
func (s *Store) Logout(ctx context.Context, hash []byte) error {
	tx, e := s.pool.Begin(ctx)
	if e != nil {
		return e
	}
	defer rollback(tx)
	q := s.q.WithTx(tx)
	token, e := q.TokenByHash(ctx, hash)
	if errors.Is(e, pgx.ErrNoRows) {
		return nil
	}
	if e != nil {
		return e
	}
	if _, e = q.LockSession(ctx, token.SessionID); e != nil {
		return storageError(e)
	}
	if e = q.RevokeSession(ctx, token.SessionID); e != nil {
		return e
	}
	return tx.Commit(ctx)
}

func (s *Store) UpdateName(ctx context.Context, userID, name string) (auth.User, error) {
	u, e := s.q.UpdateUserName(ctx, sqlc.UpdateUserNameParams{ID: uuid(userID), Name: name})
	if e != nil {
		return auth.User{}, storageError(e)
	}
	return user(u), nil
}

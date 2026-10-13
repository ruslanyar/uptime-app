// Package admin manages administrative access to user accounts.
package admin

import (
	"context"
	"errors"
	"strings"
	"uptime-app/backend/internal/auth"
)

var (
	ErrForbidden = errors.New("administrator role required")
	ErrNotFound  = errors.New("user not found")
	ErrProtected = errors.New("administrator cannot be deleted")
)

const PageSize = 25

type User struct {
	ID    string `json:"id" binding:"required" format:"uuid"`
	Name  string `json:"name" binding:"required"`
	Email string `json:"email" binding:"required"`
	Role  string `json:"role" binding:"required" enums:"user,admin"`
}

type List struct {
	Users    []User `json:"users" binding:"required"`
	Total    int64  `json:"total" binding:"required"`
	Page     int32  `json:"page" binding:"required"`
	PageSize int32  `json:"page_size" binding:"required"`
}

type Store interface {
	ListUsers(context.Context, int32) (List, error)
	DeleteUser(context.Context, string, string) (string, error)
	GrantAdmin(context.Context, string) error
}

type Service struct{ store Store }

func NewService(store Store) *Service { return &Service{store: store} }

// List returns one page of public account fields; the caller must authenticate an admin.
func (s *Service) List(ctx context.Context, page int32) (List, error) {
	if page < 1 {
		return List{}, auth.ErrInvalid
	}
	return s.store.ListUsers(ctx, page)
}

// Delete rechecks the actor and target roles atomically; returns the removed avatar URL after commit.
func (s *Service) Delete(ctx context.Context, actor, target string) (string, error) {
	if !auth.ValidID(actor) || !auth.ValidID(target) {
		return "", auth.ErrInvalid
	}
	// PostgreSQL UUIDs ignore letter case; use one ordering for every row lock.
	actor, target = strings.ToLower(actor), strings.ToLower(target)
	if actor == target {
		return "", ErrProtected
	}
	return s.store.DeleteUser(ctx, actor, target)
}

// Grant assigns admin to an existing normalized email; repeated grants succeed.
func (s *Service) Grant(ctx context.Context, email string) error {
	email, err := auth.NormalizeEmail(email)
	if err != nil {
		return err
	}
	return s.store.GrantAdmin(ctx, email)
}

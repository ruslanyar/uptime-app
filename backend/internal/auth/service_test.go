package auth

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

type stubStore struct {
	account Account
	session Session
	hash    []byte
	err     error
}

func (s *stubStore) Register(_ context.Context, a Account, session Session, hash []byte) error {
	s.account = a
	s.session = session
	s.hash = hash
	return s.err
}
func (s *stubStore) ByEmail(context.Context, string) (Account, error) { return s.account, s.err }
func (s *stubStore) ByID(context.Context, string) (User, error)       { return s.account.User, s.err }
func (s *stubStore) CreateSession(_ context.Context, session Session, hash []byte) error {
	s.session = session
	s.hash = hash
	return s.err
}
func (s *stubStore) Rotate(_ context.Context, old, newHash []byte) (User, Session, error) {
	s.hash = newHash
	return s.account.User, s.session, s.err
}
func (s *stubStore) Logout(_ context.Context, hash []byte) error { s.hash = hash; return s.err }
func TestService(t *testing.T) {
	ctx := t.Context()
	store := &stubStore{}
	tokens := NewTokens(strings.Repeat("s", 32), "issuer", "audience")
	service, e := NewService(store, tokens)
	if e != nil {
		t.Fatal(e)
	}
	password := "  password with spaces  "
	r, e := service.Register(ctx, " Name ", " USER@EXAMPLE.com ", password)
	if e != nil {
		t.Fatal(e)
	}
	if r.User != store.account.User || r.User.Name != "Name" || r.User.Email != "user@example.com" || !CheckPassword(store.account.PasswordHash, password) || !bytes.Equal(store.hash, TokenHash(r.RefreshToken)) || store.session.UserID != r.User.ID {
		t.Fatal("invalid persisted registration")
	}
	if delta := time.Until(store.session.ExpiresAt); delta > SessionTTL || delta < SessionTTL-time.Second {
		t.Fatal("session lifetime", delta)
	}
	id, e := tokens.Verify(r.AccessToken)
	if e != nil || id != r.User.ID {
		t.Fatal("access identity", e)
	}
	login, e := service.Login(ctx, r.User.Email, password)
	if e != nil || login.RefreshToken == r.RefreshToken {
		t.Fatal("independent login", e)
	}
	refreshed, e := service.Refresh(ctx, login.RefreshToken)
	if e != nil || refreshed.SessionExpires != login.SessionExpires || !bytes.Equal(store.hash, TokenHash(refreshed.RefreshToken)) {
		t.Fatal("refresh contract", e)
	}
	if e = service.Logout(ctx, refreshed.RefreshToken); e != nil || !bytes.Equal(store.hash, TokenHash(refreshed.RefreshToken)) {
		t.Fatal("logout contract", e)
	}
	if u, e := service.Me(ctx, r.AccessToken); e != nil || u != r.User {
		t.Fatal(u, e)
	}
	_, wrong := service.Login(ctx, r.User.Email, "wrong password here")
	store.err = ErrUnauthorized
	_, unknown := service.Login(ctx, "unknown@example.com", password)
	if !errors.Is(wrong, ErrUnauthorized) || !errors.Is(unknown, ErrUnauthorized) {
		t.Fatal("login error mismatch")
	}
	store.err = errors.New("commit failed")
	if r, e = service.Register(ctx, "Name", "new@example.com", password); e == nil || r.AccessToken != "" || r.RefreshToken != "" {
		t.Fatal("registration returned tokens on failure")
	}
	if r, e = service.Login(ctx, "user@example.com", password); e == nil || r.AccessToken != "" {
		t.Fatal("login masked store error")
	}
	if r, e = service.Refresh(ctx, login.RefreshToken); e == nil || r.RefreshToken != "" {
		t.Fatal("refresh returned token on failure")
	}
	if _, e = service.Refresh(ctx, "malformed"); !errors.Is(e, ErrUnauthorized) {
		t.Fatal(e)
	}
	if _, e = service.Me(ctx, "malformed"); !errors.Is(e, ErrUnauthorized) {
		t.Fatal(e)
	}
	if e = service.Logout(ctx, ""); e != nil {
		t.Fatal("empty logout not idempotent", e)
	}
}

func (s *stubStore) UpdateName(_ context.Context, id, name string) (User, error) {
	if id != s.account.ID {
		return User{}, ErrUnauthorized
	}
	if s.err != nil {
		return User{}, s.err
	}
	s.account.Name = name
	return s.account.User, nil
}
func TestUpdateProfile(t *testing.T) {
	store := &stubStore{account: Account{User: User{ID: NewID(), Name: "Old", Email: "user@example.com"}}}
	tokens := NewTokens(strings.Repeat("s", 32), "issuer", "audience")
	service, e := NewService(store, tokens)
	if e != nil {
		t.Fatal(e)
	}
	token, e := tokens.Issue(store.account.ID)
	if e != nil {
		t.Fatal(e)
	}
	u, e := service.UpdateProfile(t.Context(), token, "  Новое имя  ")
	if e != nil || u.Name != "Новое имя" || u.Email != "user@example.com" {
		t.Fatal(u, e)
	}
	for _, name := range []string{"", "   ", "я", strings.Repeat("я", 51), string([]byte{0xff})} {
		if _, e := service.UpdateProfile(t.Context(), token, name); !errors.Is(e, ErrInvalid) {
			t.Fatal(name, e)
		}
	}
	if _, e := service.UpdateProfile(t.Context(), "invalid", "Name"); !errors.Is(e, ErrUnauthorized) {
		t.Fatal(e)
	}
}

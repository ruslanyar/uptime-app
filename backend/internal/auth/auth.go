package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"net/mail"
	"strings"
	"time"
	"unicode/utf8"
)

var (
	ErrInvalid      = errors.New("invalid input")
	ErrUnauthorized = errors.New("invalid credentials or token")
	ErrConflict     = errors.New("email already registered")
)

const AccessTTL = 24 * time.Hour
const SessionTTL = 30 * 24 * time.Hour

type User struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}
type Account struct {
	User
	PasswordHash string
}
type Session struct {
	ID, UserID string
	ExpiresAt  time.Time
}
type Store interface {
	Register(context.Context, Account, Session, []byte) error
	ByEmail(context.Context, string) (Account, error)
	ByID(context.Context, string) (User, error)
	CreateSession(context.Context, Session, []byte) error
	Rotate(context.Context, []byte, []byte) (User, Session, error)
	Logout(context.Context, []byte) error
}
type Result struct {
	AccessToken    string    `json:"access_token"`
	TokenType      string    `json:"token_type"`
	ExpiresIn      int       `json:"expires_in"`
	User           User      `json:"user"`
	RefreshToken   string    `json:"-"`
	SessionExpires time.Time `json:"-"`
}
type Service struct {
	store     Store
	tokens    *Tokens
	dummyHash string
}

func NewService(store Store, tokens *Tokens) (*Service, error) {
	h, e := HashPassword("dummy password for timing")
	return &Service{store: store, tokens: tokens, dummyHash: h}, e
}
func NormalizeEmail(s string) (string, error) {
	s = strings.ToLower(strings.TrimSpace(s))
	a, e := mail.ParseAddress(s)
	if e != nil || a.Address != s || len(s) > 254 || !strings.Contains(s, "@") || strings.ContainsAny(s, "\r\n") {
		return "", ErrInvalid
	}
	return s, nil
}
func Validate(name, email, password string) (string, string, error) {
	name = strings.TrimSpace(name)
	email, e := NormalizeEmail(email)
	if e != nil || !utf8.ValidString(name) || !validPassword(password) || utf8.RuneCountInString(name) < 1 || utf8.RuneCountInString(name) > 100 {
		return "", "", ErrInvalid
	}
	return name, email, nil
}
func validPassword(password string) bool {
	return utf8.ValidString(password) && utf8.RuneCountInString(password) >= 15 && utf8.RuneCountInString(password) <= 128
}
func NewID() string {
	b := make([]byte, 16)
	if _, e := rand.Read(b); e != nil {
		panic(e)
	}
	b[6] = (b[6] & 15) | 64
	b[8] = (b[8] & 63) | 128
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[:4], b[4:6], b[6:8], b[8:10], b[10:])
}
func ValidID(s string) bool {
	if len(s) != 36 {
		return false
	}
	for i, c := range s {
		if i == 8 || i == 13 || i == 18 || i == 23 {
			if c != '-' {
				return false
			}
		} else if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return false
		}
	}
	return true
}
func refresh() (string, []byte, error) {
	b := make([]byte, 32)
	if _, e := rand.Read(b); e != nil {
		return "", nil, e
	}
	s := base64.RawURLEncoding.EncodeToString(b)
	return s, TokenHash(s), nil
}
func TokenHash(s string) []byte { h := sha256.Sum256([]byte(s)); return h[:] }
func validRefresh(s string) bool {
	b, e := base64.RawURLEncoding.DecodeString(s)
	return e == nil && len(b) == 32 && base64.RawURLEncoding.EncodeToString(b) == s
}
func (s *Service) result(u User, session Session, raw, access string) Result {
	return Result{access, "Bearer", int(AccessTTL.Seconds()), u, raw, session.ExpiresAt}
}
func (s *Service) Register(ctx context.Context, name, email, password string) (Result, error) {
	name, email, e := Validate(name, email, password)
	if e != nil {
		return Result{}, e
	}
	h, e := HashPassword(password)
	if e != nil {
		return Result{}, e
	}
	u := User{NewID(), name, email}
	session := Session{NewID(), u.ID, time.Now().Add(SessionTTL)}
	raw, hash, e := refresh()
	if e != nil {
		return Result{}, e
	}
	access, e := s.tokens.Issue(u.ID)
	if e != nil {
		return Result{}, e
	}
	if e = s.store.Register(ctx, Account{u, h}, session, hash); e != nil {
		return Result{}, e
	}
	return s.result(u, session, raw, access), nil
}
func (s *Service) Login(ctx context.Context, email, password string) (Result, error) {
	email, e := NormalizeEmail(email)
	if e != nil || !validPassword(password) {
		return Result{}, ErrInvalid
	}
	a, e := s.store.ByEmail(ctx, email)
	if errors.Is(e, ErrUnauthorized) {
		CheckPassword(s.dummyHash, password)
		return Result{}, ErrUnauthorized
	}
	if e != nil {
		return Result{}, e
	}
	if !CheckPassword(a.PasswordHash, password) {
		return Result{}, ErrUnauthorized
	}
	session := Session{NewID(), a.ID, time.Now().Add(SessionTTL)}
	raw, hash, e := refresh()
	if e != nil {
		return Result{}, e
	}
	access, e := s.tokens.Issue(a.ID)
	if e != nil {
		return Result{}, e
	}
	if e = s.store.CreateSession(ctx, session, hash); e != nil {
		return Result{}, e
	}
	return s.result(a.User, session, raw, access), nil
}
func (s *Service) Refresh(ctx context.Context, old string) (Result, error) {
	if !validRefresh(old) {
		return Result{}, ErrUnauthorized
	}
	raw, hash, e := refresh()
	if e != nil {
		return Result{}, e
	}
	u, session, e := s.store.Rotate(ctx, TokenHash(old), hash)
	if e != nil {
		return Result{}, e
	}
	access, e := s.tokens.Issue(u.ID)
	if e != nil {
		return Result{}, e
	}
	return s.result(u, session, raw, access), nil
}
func (s *Service) Logout(ctx context.Context, raw string) error {
	if !validRefresh(raw) {
		return nil
	}
	return s.store.Logout(ctx, TokenHash(raw))
}
func (s *Service) Me(ctx context.Context, raw string) (User, error) {
	id, e := s.tokens.Verify(raw)
	if e != nil {
		return User{}, ErrUnauthorized
	}
	return s.store.ByID(ctx, id)
}

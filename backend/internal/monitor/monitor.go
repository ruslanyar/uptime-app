// Package monitor manages user-owned monitoring configurations.
package monitor

import (
	"context"
	"errors"
	"net/netip"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
	"uptime-app/backend/internal/auth"
)

var ErrInvalid = errors.New("invalid monitor input")
var ErrConflict = errors.New("monitor already exists")
var ErrNotFound = errors.New("monitor not found")

type Monitor struct {
	ID              string    `json:"id" binding:"required" format:"uuid"`
	UserID          string    `json:"-"`
	URL             string    `json:"url" binding:"required"`
	IntervalSeconds int32     `json:"interval_seconds" binding:"required"`
	CreatedAt       time.Time `json:"created_at" binding:"required" format:"date-time"`
	UpdatedAt       time.Time `json:"updated_at" binding:"required" format:"date-time"`
}
type Store interface {
	CreateMonitor(context.Context, Monitor) (Monitor, error)
	ListMonitors(context.Context, string) ([]Monitor, error)
	UpdateMonitor(context.Context, Monitor) (Monitor, error)
	DeleteMonitor(context.Context, string, string) error
}

// Service expects an authenticated userID; Store must enforce ownership.
// Cancellation and deadlines are delegated to Store.
type Service struct{ store Store }

func NewService(store Store) *Service { return &Service{store: store} }

// Validate preserves the trimmed URL verbatim because uniqueness uses exact strings.
func Validate(raw string, interval int32) (string, error) {
	raw = strings.TrimSpace(raw)
	if len(raw) == 0 || len(raw) > 2048 || !utf8.ValidString(raw) || interval < 60 || interval > 86400 || strings.Contains(raw, "#") {
		return "", ErrInvalid
	}
	u, e := url.Parse(raw)
	if e != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Opaque != "" || u.Hostname() == "" || u.User != nil || strings.ContainsAny(raw, " \t\r\n") {
		return "", ErrInvalid
	}
	if strings.HasPrefix(u.Host, "[") {
		// URL parsing alone does not guarantee that a bracketed host is valid IPv6.
		addr, err := netip.ParseAddr(u.Hostname())
		if err != nil || !addr.Is6() {
			return "", ErrInvalid
		}
	} else if strings.Contains(u.Hostname(), ":") {
		return "", ErrInvalid
	}
	if strings.HasSuffix(u.Host, ":") {
		return "", ErrInvalid
	}
	if port := u.Port(); port != "" {
		n, e := strconv.Atoi(port)
		if e != nil || n < 1 || n > 65535 {
			return "", ErrInvalid
		}
	}
	return raw, nil
}
func (s *Service) Create(ctx context.Context, userID, raw string, interval int32) (Monitor, error) {
	raw, e := Validate(raw, interval)
	if e != nil || !auth.ValidID(userID) {
		return Monitor{}, ErrInvalid
	}
	return s.store.CreateMonitor(ctx, Monitor{ID: auth.NewID(), UserID: userID, URL: raw, IntervalSeconds: interval})
}

func (s *Service) List(ctx context.Context, userID string) ([]Monitor, error) {
	if !auth.ValidID(userID) {
		return nil, ErrInvalid
	}
	return s.store.ListMonitors(ctx, userID)
}

func (s *Service) Update(ctx context.Context, userID, monitorID, raw string, interval int32) (Monitor, error) {
	raw, err := Validate(raw, interval)
	if err != nil || !auth.ValidID(userID) || !auth.ValidID(monitorID) {
		return Monitor{}, ErrInvalid
	}
	return s.store.UpdateMonitor(ctx, Monitor{ID: monitorID, UserID: userID, URL: raw, IntervalSeconds: interval})
}

func (s *Service) Delete(ctx context.Context, userID, monitorID string) error {
	if !auth.ValidID(userID) || !auth.ValidID(monitorID) {
		return ErrInvalid
	}
	return s.store.DeleteMonitor(ctx, userID, monitorID)
}

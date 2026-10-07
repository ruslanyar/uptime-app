package monitor

import (
	"context"
	"errors"
	"strings"
	"testing"
	"uptime-app/backend/internal/auth"
)

func TestValidate(t *testing.T) {
	for _, raw := range []string{"https://example.com", " http://localhost:8080/path?q=1 ", "http://127.0.0.1", "https://[::1]:443/", "HTTPS://Example.COM/?x=1"} {
		for _, n := range []int32{60, 86400} {
			got, e := Validate(raw, n)
			if e != nil || got != strings.TrimSpace(raw) {
				t.Fatal(raw, n, got, e)
			}
		}
	}
	for _, raw := range []string{"", "example.com", "ftp://example.com", "https:///path", "https://user:pass@example.com", "https://example.com/#x", "https://example.com/#", "https://example.com:0", "https://example.com:65536", "https://example.com:", "https://example.com:no", "https://[invalid]/", "https://example.com:80:90/", "https://example.com/%zz", "https://example.com/a b", "https://example.com/" + strings.Repeat("x", 2048)} {
		if _, e := Validate(raw, 300); !errors.Is(e, ErrInvalid) {
			t.Fatal(raw, e)
		}
	}
	for _, n := range []int32{-1, 0, 59, 86401} {
		if _, e := Validate("https://example.com", n); !errors.Is(e, ErrInvalid) {
			t.Fatal(n, e)
		}
	}
}

type fakeStore struct {
	calls int
	value Monitor
	err   error
}

func (f *fakeStore) CreateMonitor(_ context.Context, m Monitor) (Monitor, error) {
	f.calls++
	f.value = m
	return m, f.err
}
func TestCreate(t *testing.T) {
	f := &fakeStore{}
	s := NewService(f)
	owner := auth.NewID()
	m, e := s.Create(context.Background(), owner, " https://example.com ", 300)
	if e != nil || m.UserID != owner || m.URL != "https://example.com" || !auth.ValidID(m.ID) {
		t.Fatal(m, e)
	}
	if _, e = s.Create(context.Background(), owner, "bad", 300); !errors.Is(e, ErrInvalid) || f.calls != 1 {
		t.Fatal(e, f.calls)
	}
	for _, err := range []error{ErrConflict, errors.New("storage failure")} {
		f.err = err
		if _, e = s.Create(context.Background(), owner, "https://example.com", 300); !errors.Is(e, err) {
			t.Fatal(e)
		}
	}
}

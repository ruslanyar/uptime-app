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

func (f *fakeStore) ListMonitors(_ context.Context, owner string) ([]Monitor, error) {
	return []Monitor{{UserID: owner}}, f.err
}
func TestList(t *testing.T) {
	f := &fakeStore{}
	s := NewService(f)
	owner := auth.NewID()
	items, e := s.List(context.Background(), owner)
	if e != nil || len(items) != 1 || items[0].UserID != owner {
		t.Fatal(items, e)
	}
	if _, e = s.List(context.Background(), "invalid"); !errors.Is(e, ErrInvalid) {
		t.Fatal(e)
	}
	f.err = errors.New("database unavailable")
	if _, e = s.List(context.Background(), owner); !errors.Is(e, f.err) {
		t.Fatal(e)
	}
}

func (f *fakeStore) UpdateMonitor(_ context.Context, m Monitor) (Monitor, error) {
	f.calls++
	f.value = m
	return m, f.err
}
func (f *fakeStore) DeleteMonitor(_ context.Context, owner, id string) error {
	f.calls++
	f.value = Monitor{ID: id, UserID: owner}
	return f.err
}

func TestUpdate(t *testing.T) {
	owner, id := auth.NewID(), auth.NewID()
	for _, tc := range []struct {
		name, owner, id, url string
		interval             int32
		invalid              bool
	}{
		{"valid", owner, id, " https://example.com ", 300, false},
		{"owner", "invalid", id, "https://example.com", 300, true},
		{"id", owner, "invalid", "https://example.com", 300, true},
		{"url", owner, id, "bad", 300, true},
		{"interval", owner, id, "https://example.com", 59, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := &fakeStore{}
			m, err := NewService(f).Update(context.Background(), tc.owner, tc.id, tc.url, tc.interval)
			if tc.invalid {
				if !errors.Is(err, ErrInvalid) || f.calls != 0 {
					t.Fatal(err, f.calls)
				}
				return
			}
			if err != nil || f.calls != 1 || m.ID != id || m.UserID != owner || m.URL != "https://example.com" || m.IntervalSeconds != 300 {
				t.Fatal(m, err, f.calls)
			}
		})
	}
	for _, want := range []error{ErrNotFound, ErrConflict, errors.New("database unavailable")} {
		f := &fakeStore{err: want}
		if _, err := NewService(f).Update(context.Background(), owner, id, "https://example.com", 300); !errors.Is(err, want) {
			t.Fatal(err)
		}
	}
}

func TestDelete(t *testing.T) {
	owner, id := auth.NewID(), auth.NewID()
	for _, ids := range [][2]string{{"invalid", id}, {owner, "invalid"}} {
		f := &fakeStore{}
		if err := NewService(f).Delete(context.Background(), ids[0], ids[1]); !errors.Is(err, ErrInvalid) || f.calls != 0 {
			t.Fatal(err, f.calls)
		}
	}
	for _, want := range []error{nil, ErrNotFound, errors.New("database unavailable")} {
		f := &fakeStore{err: want}
		err := NewService(f).Delete(context.Background(), owner, id)
		if !errors.Is(err, want) || f.calls != 1 || f.value.ID != id || f.value.UserID != owner {
			t.Fatal(err, f)
		}
	}
}

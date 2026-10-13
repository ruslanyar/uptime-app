package httpapi

import (
	"context"
	"errors"
	"net/http/httptest"
	"testing"
	"time"

	healthcontroller "uptime-app/backend/internal/controllers/health"
)

func TestReadiness(t *testing.T) {
	for _, tc := range []struct {
		name   string
		check  healthcontroller.Check
		status int
		body   string
	}{
		{"ready", func(context.Context) error { return nil }, 200, "{\"status\":\"ready\"}\n"},
		{"failed", func(context.Context) error { return errors.New("secret connection details") }, 503, "{\"status\":\"not_ready\"}\n"},
		{"missing", nil, 503, "{\"status\":\"not_ready\"}\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			auth := &fakeService{}
			h := New(auth, nil, nil, tc.check, cfg())
			w := httptest.NewRecorder()
			r := httptest.NewRequest("GET", "/api/v1/health/ready", nil)
			r.Header.Set("Authorization", "invalid")
			h.ServeHTTP(w, r)
			if w.Code != tc.status || w.Body.String() != tc.body || auth.calls != 0 || w.Header().Get("Cache-Control") != "no-store" || w.Header().Get("Content-Type") != "application/json" || len(w.Result().Cookies()) != 0 {
				t.Fatalf("response: %d %s %v", w.Code, w.Body.String(), w.Header())
			}
		})
	}
}

func TestReadinessDeadlineAndCancellation(t *testing.T) {
	for _, canceled := range []bool{false, true} {
		ctx, cancel := context.WithCancel(context.Background())
		if canceled {
			cancel()
		}
		h := New(nil, nil, nil, func(checkCtx context.Context) error {
			deadline, ok := checkCtx.Deadline()
			if !ok || time.Until(deadline) > time.Second {
				t.Fatal("missing readiness budget")
			}
			if canceled {
				if !errors.Is(checkCtx.Err(), context.Canceled) {
					t.Fatal("request cancellation lost")
				}
				return nil
			}
			<-checkCtx.Done()
			return checkCtx.Err()
		}, cfg())
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest("GET", "/api/v1/health/ready", nil).WithContext(ctx))
		cancel()
		if w.Code != 503 {
			t.Fatal(w.Code)
		}
	}
}

func TestReadinessProtectionAndHead(t *testing.T) {
	calls := 0
	h := New(nil, nil, nil, func(context.Context) error { calls++; return nil }, cfg())
	r := httptest.NewRequest("GET", "/api/v1/health/ready", nil)
	r.Header.Set("Origin", "https://unknown.example")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 403 || calls != 0 {
		t.Fatal(w.Code, calls)
	}
	server := httptest.NewServer(h)
	defer server.Close()
	resp, err := server.Client().Head(server.URL + "/api/v1/health/ready")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 || resp.ContentLength < 0 || calls != 1 {
		t.Fatal(resp.StatusCode, calls)
	}
	r = httptest.NewRequest("POST", "/api/v1/health/ready", nil)
	r.Header.Set("X-CSRF-Protection", "1")
	w = httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 405 || calls != 1 {
		t.Fatal(w.Code, calls)
	}
}

package monitorcontroller

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"uptime-app/backend/internal/auth"
	"uptime-app/backend/internal/monitor"
)

type fakeAuth struct {
	token string
	err   error
}

func (f *fakeAuth) Me(_ context.Context, token string) (auth.User, error) {
	f.token = token
	return auth.User{ID: "owner"}, f.err
}

type fakeMonitor struct {
	calls int
	owner string
	err   error
}

func (f *fakeMonitor) Create(_ context.Context, owner, url string, n int32) (monitor.Monitor, error) {
	f.calls++
	f.owner = owner
	return monitor.Monitor{ID: "id", URL: url, IntervalSeconds: n}, f.err
}
func TestCreateHTTP(t *testing.T) {
	valid := `{"url":"https://example.com","interval_seconds":300}`
	for _, tc := range []struct {
		name, body, media, cookie string
		headers                   []string
		authErr, storeErr         error
		status                    int
	}{
		{name: "cookie", body: valid, cookie: "cookie", status: 201},
		{name: "bearer", body: valid, headers: []string{"Bearer header"}, status: 201},
		{name: "precedence", body: valid, cookie: "cookie", headers: []string{"Bearer header"}, status: 201},
		{name: "missing", body: valid, status: 401},
		{name: "invalid header", body: valid, cookie: "cookie", headers: []string{"Basic bad"}, status: 401},
		{name: "empty header", body: valid, cookie: "cookie", headers: []string{""}, status: 401},
		{name: "duplicate headers", body: valid, cookie: "cookie", headers: []string{"Bearer one", "Bearer two"}, status: 401},
		{name: "expired", body: valid, cookie: "cookie", authErr: auth.ErrUnauthorized, status: 401},
		{name: "auth storage failure", body: valid, cookie: "cookie", authErr: errors.New("secret database error"), status: 500},
		{name: "conflict", body: valid, cookie: "cookie", storeErr: monitor.ErrConflict, status: 409},
		{name: "invalid", body: valid, cookie: "cookie", storeErr: monitor.ErrInvalid, status: 400},
		{name: "storage failure", body: valid, cookie: "cookie", storeErr: errors.New("secret database error"), status: 500},
		{name: "media", body: valid, media: "text/plain", cookie: "cookie", status: 400},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a := &fakeAuth{err: tc.authErr}
			m := &fakeMonitor{err: tc.storeErr}
			r := httptest.NewRequest("POST", "/api/v1/monitors", strings.NewReader(tc.body))
			r.Header.Set("Content-Type", "application/json")
			if tc.media != "" {
				r.Header.Set("Content-Type", tc.media)
			}
			if tc.cookie != "" {
				r.AddCookie(&http.Cookie{Name: "access_token", Value: tc.cookie})
			}
			if tc.headers != nil {
				r.Header["Authorization"] = tc.headers
			}
			w := httptest.NewRecorder()
			New(a, m).Create(w, r)
			if w.Code != tc.status || w.Header().Get("Cache-Control") != "no-store" || strings.Contains(w.Body.String(), "secret") {
				t.Fatal(w.Code, w.Body.String())
			}
			if tc.status == 201 && (m.owner != "owner" || m.calls != 1) {
				t.Fatal(m)
			}
			if tc.name == "precedence" && a.token != "header" {
				t.Fatal(a.token)
			}
		})
	}
	for _, body := range []string{"", `null`, `[]`, `{`, `{} {}`, `{"url":"https://example.com","interval_seconds":300,"user_id":"other"}`, `{"interval_seconds":1.5}`, `{"interval_seconds":"300"}`, `{"interval_seconds":2147483648}`, `{"url":"https://example.com/` + strings.Repeat("x", 17000) + `","interval_seconds":300}`} {
		m := &fakeMonitor{}
		r := httptest.NewRequest("POST", "/api/v1/monitors", strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		r.AddCookie(&http.Cookie{Name: "access_token", Value: "cookie"})
		w := httptest.NewRecorder()
		New(&fakeAuth{}, m).Create(w, r)
		if w.Code != 400 || m.calls != 0 {
			t.Fatal(body[:min(30, len(body))], w.Code, m.calls)
		}
	}
}

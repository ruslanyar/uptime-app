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
	id    string
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

func (f *fakeMonitor) List(_ context.Context, owner string) ([]monitor.Monitor, error) {
	f.calls++
	f.owner = owner
	return nil, f.err
}
func TestListHTTP(t *testing.T) {
	for _, tc := range []struct {
		token  string
		err    error
		status int
	}{{"", nil, 401}, {"access", nil, 200}, {"access", errors.New("database unavailable"), 500}} {
		m := &fakeMonitor{err: tc.err}
		r := httptest.NewRequest("GET", "/api/v1/monitors", nil)
		if tc.token != "" {
			r.Header.Set("Authorization", "Bearer "+tc.token)
		}
		w := httptest.NewRecorder()
		New(&fakeAuth{}, m).List(w, r)
		if w.Code != tc.status || w.Header().Get("Cache-Control") != "no-store" {
			t.Fatal(w.Code, w.Body.String())
		}
		if tc.status == 200 && (strings.TrimSpace(w.Body.String()) != `{"monitors":[]}` || m.owner != "owner") {
			t.Fatal(w.Body.String(), m.owner)
		}
	}
}

func (f *fakeMonitor) Update(_ context.Context, owner, id, url string, interval int32) (monitor.Monitor, error) {
	f.calls++
	f.owner = owner
	f.id = id
	return monitor.Monitor{ID: id, URL: url, IntervalSeconds: interval}, f.err
}
func (f *fakeMonitor) Delete(_ context.Context, owner, id string) error {
	f.calls++
	f.owner = owner
	f.id = id
	return f.err
}

func TestMutationHTTP(t *testing.T) {
	for _, method := range []string{"PUT", "DELETE"} {
		for _, tc := range []struct {
			name, token         string
			authErr, serviceErr error
			status              int
		}{
			{name: "success", token: "access", status: 200},
			{name: "missing auth", status: 401},
			{name: "expired", token: "access", authErr: auth.ErrUnauthorized, status: 401},
			{name: "auth failure", token: "access", authErr: errors.New("secret database error"), status: 500},
			{name: "invalid", token: "access", serviceErr: monitor.ErrInvalid, status: 400},
			{name: "not found", token: "access", serviceErr: monitor.ErrNotFound, status: 404},
			{name: "conflict", token: "access", serviceErr: monitor.ErrConflict, status: 409},
			{name: "storage failure", token: "access", serviceErr: errors.New("secret database error"), status: 500},
		} {
			t.Run(method+"/"+tc.name, func(t *testing.T) {
				m := &fakeMonitor{err: tc.serviceErr}
				r := httptest.NewRequest(method, "/api/v1/monitors/id", strings.NewReader(`{"url":"https://example.com","interval_seconds":300}`))
				r.SetPathValue("id", "id")
				r.Header.Set("Content-Type", "application/json")
				if tc.token != "" {
					r.AddCookie(&http.Cookie{Name: "access_token", Value: tc.token})
				}
				w := httptest.NewRecorder()
				c := New(&fakeAuth{err: tc.authErr}, m)
				if method == "PUT" {
					c.Update(w, r)
				} else {
					c.Delete(w, r)
				}
				want := tc.status
				if method == "DELETE" && want == 200 {
					want = 204
				}
				if w.Code != want || w.Header().Get("Cache-Control") != "no-store" || strings.Contains(w.Body.String(), "secret") {
					t.Fatal(w.Code, w.Body.String())
				}
				if tc.token == "" || tc.authErr != nil {
					if m.calls != 0 {
						t.Fatal("unauthorized request reached monitor service")
					}
				} else if m.calls != 1 || m.owner != "owner" || m.id != "id" {
					t.Fatal(m)
				}
				if want == 204 && w.Body.Len() != 0 {
					t.Fatal(w.Body.String())
				}
				if want == 404 && !strings.Contains(w.Body.String(), "monitor_not_found") {
					t.Fatal(w.Body.String())
				}
			})
		}
	}
}

func TestUpdateRejectsMalformedJSON(t *testing.T) {
	for _, body := range []string{"", `null`, `[]`, `{`, `{} {}`, `{"url":"https://example.com","interval_seconds":300,"updated_at":"today"}`, `{"interval_seconds":1.5}`, `{"interval_seconds":"300"}`, `{"interval_seconds":2147483648}`, strings.Repeat("x", 17000)} {
		m := &fakeMonitor{}
		r := httptest.NewRequest("PUT", "/api/v1/monitors/id", strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("Authorization", "Bearer access")
		w := httptest.NewRecorder()
		New(&fakeAuth{}, m).Update(w, r)
		if w.Code != 400 || m.calls != 0 {
			t.Fatal(w.Code, m.calls)
		}
	}
}

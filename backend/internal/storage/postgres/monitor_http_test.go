package postgres_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"uptime-app/backend/internal/config"
	"uptime-app/backend/internal/httpapi"
	"uptime-app/backend/internal/monitor"
	"uptime-app/backend/internal/storage/postgres"
)

func TestMonitorRoute(t *testing.T) {
	ctx, pool, a, _ := setup(t)
	u := register(t, ctx, a)
	h := httpapi.New(a, monitor.NewService(postgres.New(pool)), config.Config{AllowedOrigins: map[string]bool{"http://localhost:3000": true}})
	post := func(token, csrf, origin, body string, header *string) *httptest.ResponseRecorder {
		r := httptest.NewRequest("POST", "/api/v1/monitors", strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("X-CSRF-Protection", csrf)
		if origin != "" {
			r.Header.Set("Origin", origin)
		}
		if token != "" {
			r.AddCookie(&http.Cookie{Name: "access_token", Value: token})
		}
		if header != nil {
			r.Header.Set("Authorization", *header)
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w
	}
	body := `{"url":"https://example.com","interval_seconds":300}`
	w := post(u.AccessToken, "1", "http://localhost:3000", body, nil)
	var m monitor.Monitor
	if e := json.Unmarshal(w.Body.Bytes(), &m); e != nil || w.Code != 201 || m.CreatedAt.IsZero() {
		t.Fatal(w.Code, w.Body.String(), e)
	}
	var fields map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &fields)
	if len(fields) != 4 {
		t.Fatal(fields)
	}
	if w = post(u.AccessToken, "1", "", body, nil); w.Code != 409 || !strings.Contains(w.Body.String(), "monitor_conflict") {
		t.Fatal(w.Code, w.Body.String())
	}
	bearer := "Bearer " + u.AccessToken
	if w = post("", "1", "", `{"url":"http://localhost:8080","interval_seconds":86400}`, &bearer); w.Code != 201 {
		t.Fatal(w.Code, w.Body.String())
	}
	for _, tc := range []struct {
		token, csrf, origin string
		header              *string
		status              int
	}{
		{csrf: "1", status: 401}, {token: "bad", csrf: "1", status: 401}, {token: expiredAccess(t, u.User.ID), csrf: "1", status: 401},
		{token: u.AccessToken, status: 403}, {token: u.AccessToken, csrf: "1", origin: "https://evil.example", status: 403},
	} {
		if w = post(tc.token, tc.csrf, tc.origin, body, tc.header); w.Code != tc.status {
			t.Fatal(w.Code, tc.status)
		}
	}
	invalid := "Bearer broken"
	if w = post(u.AccessToken, "1", "", body, &invalid); w.Code != 401 {
		t.Fatal(w.Code)
	}
	for _, body := range []string{`{}`, `{"url":"bad","interval_seconds":300}`, `{"url":"https://invalid.example","interval_seconds":59}`, `{"url":"https://invalid.example","interval_seconds":null}`} {
		if w = post(u.AccessToken, "1", "", body, nil); w.Code != 400 {
			t.Fatal(w.Code, w.Body.String())
		}
	}
	var count int
	if e := pool.QueryRow(ctx, "SELECT count(*) FROM monitors").Scan(&count); e != nil || count != 2 {
		t.Fatal(count, e)
	}
	if _, e := pool.Exec(ctx, "DROP TABLE monitors"); e != nil {
		t.Fatal(e)
	}
	if w = post(u.AccessToken, "1", "", body, nil); w.Code != 500 || !strings.Contains(w.Body.String(), "internal_error") {
		t.Fatal(w.Code, w.Body.String())
	}
}

func TestMonitorListRoute(t *testing.T) {
	ctx, pool, a, _ := setup(t)
	first := register(t, ctx, a)
	second, e := a.Register(ctx, "Other", "other@example.com", "long password here")
	if e != nil {
		t.Fatal(e)
	}
	s := monitor.NewService(postgres.New(pool))
	h := httpapi.New(a, s, config.Config{})
	get := func(token string) *httptest.ResponseRecorder {
		r := httptest.NewRequest("GET", "/api/v1/monitors", nil)
		if token != "" {
			r.AddCookie(&http.Cookie{Name: "access_token", Value: token})
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w
	}
	if w := get(""); w.Code != 401 {
		t.Fatal(w.Code)
	}
	if w := get(first.AccessToken); w.Code != 200 || strings.TrimSpace(w.Body.String()) != `{"monitors":[]}` {
		t.Fatal(w.Code, w.Body.String())
	}
	old, e := s.Create(ctx, first.User.ID, "https://old.example", 300)
	if e != nil {
		t.Fatal(e)
	}
	recent, e := s.Create(ctx, first.User.ID, "https://new.example", 60)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = s.Create(ctx, second.User.ID, "https://private.example", 300); e != nil {
		t.Fatal(e)
	}
	w := get(first.AccessToken)
	var payload struct {
		Monitors []monitor.Monitor `json:"monitors"`
	}
	if e = json.Unmarshal(w.Body.Bytes(), &payload); e != nil || w.Code != 200 || len(payload.Monitors) != 2 {
		t.Fatal(w.Code, w.Body.String(), e)
	}
	if payload.Monitors[0].ID != recent.ID || payload.Monitors[1].ID != old.ID || w.Header().Get("Cache-Control") != "no-store" || strings.Contains(w.Body.String(), "user_id") {
		t.Fatal(w.Body.String())
	}
	if _, e = pool.Exec(ctx, "DROP TABLE monitors"); e != nil {
		t.Fatal(e)
	}
	if w = get(first.AccessToken); w.Code != 500 {
		t.Fatal(w.Code)
	}
}

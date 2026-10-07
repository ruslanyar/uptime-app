package postgres_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"uptime-app/backend/internal/auth"
	"uptime-app/backend/internal/config"
	"uptime-app/backend/internal/httpapi"
	"uptime-app/backend/internal/monitor"
	"uptime-app/backend/internal/storage/postgres"
)

func TestMonitorMutationRoutes(t *testing.T) {
	ctx, pool, a, _ := setup(t)
	owner := register(t, ctx, a)
	other, err := a.Register(ctx, "Other", "other@example.com", "long password here")
	if err != nil {
		t.Fatal(err)
	}
	s := monitor.NewService(postgres.New(pool))
	m, err := s.Create(ctx, owner.User.ID, "https://original.example", 300)
	if err != nil {
		t.Fatal(err)
	}
	duplicate, err := s.Create(ctx, owner.User.ID, "https://duplicate.example", 60)
	if err != nil {
		t.Fatal(err)
	}
	// A known timestamp makes advancement independent of clock resolution.
	old := time.Date(2025, 1, 2, 3, 4, 5, 0, time.UTC)
	if _, err = pool.Exec(ctx, "UPDATE monitors SET created_at=$2,updated_at=$2 WHERE id=$1", m.ID, old); err != nil {
		t.Fatal(err)
	}
	m.CreatedAt, m.UpdatedAt = old, old
	h := httpapi.New(a, s, config.Config{AllowedOrigins: map[string]bool{"https://client.example": true}})
	call := func(method, id, token, csrf, origin, body, media string, bearer bool) *httptest.ResponseRecorder {
		t.Helper()
		r := httptest.NewRequest(method, "/api/v1/monitors/"+id, strings.NewReader(body))
		if token != "" {
			if bearer {
				r.Header.Set("Authorization", "Bearer "+token)
			} else {
				r.AddCookie(&http.Cookie{Name: "access_token", Value: token})
			}
		}
		if csrf != "" {
			r.Header.Set("X-CSRF-Protection", csrf)
		}
		if origin != "" {
			r.Header.Set("Origin", origin)
		}
		if media != "" {
			r.Header.Set("Content-Type", media)
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w
	}
	unchanged := func(want monitor.Monitor) {
		t.Helper()
		items, err := s.List(ctx, owner.User.ID)
		if err != nil {
			t.Fatal(err)
		}
		for _, item := range items {
			if item.ID == want.ID {
				if item != want {
					t.Fatalf("monitor changed: got %+v, want %+v", item, want)
				}
				return
			}
		}
		t.Fatal("monitor disappeared")
	}
	body := `{"url":"https://updated.example/path?q=1","interval_seconds":90}`
	for _, method := range []string{"PUT", "DELETE"} {
		for _, tc := range []struct {
			name, id, token, csrf, origin, body, media string
			status                                     int
		}{
			{"missing auth", m.ID, "", "1", "", body, "application/json", 401},
			{"bad auth", m.ID, "bad", "1", "", body, "application/json", 401},
			{"expired auth", m.ID, expiredAccess(t, owner.User.ID), "1", "", body, "application/json", 401},
			{"missing csrf", m.ID, owner.AccessToken, "", "", body, "application/json", 403},
			{"bad origin", m.ID, owner.AccessToken, "1", "https://evil.example", body, "application/json", 403},
			{"other owner", m.ID, other.AccessToken, "1", "", body, "application/json", 404},
			{"absent", auth.NewID(), owner.AccessToken, "1", "", body, "application/json", 404},
			{"invalid id", "bad", owner.AccessToken, "1", "", body, "application/json", 400},
		} {
			t.Run(method+"/"+tc.name, func(t *testing.T) {
				w := call(method, tc.id, tc.token, tc.csrf, tc.origin, tc.body, tc.media, false)
				if w.Code != tc.status || len(w.Result().Cookies()) != 0 {
					t.Fatal(w.Code, w.Body.String())
				}
				if tc.status == 404 && !strings.Contains(w.Body.String(), "monitor_not_found") {
					t.Fatal(w.Body.String())
				}
				unchanged(m)
			})
		}
	}
	for _, tc := range []struct {
		body, media string
		status      int
	}{
		{`{}`, "application/json", 400},
		{`{"url":"https://valid.example"}`, "application/json", 400},
		{`{"interval_seconds":60}`, "application/json", 400},
		{`{"url":"bad","interval_seconds":300}`, "application/json", 400},
		{`{"url":"https://valid.example","interval_seconds":59}`, "application/json", 400},
		{`{"url":"https://valid.example","interval_seconds":86401}`, "application/json", 400},
		{`{"url":"https://valid.example","interval_seconds":null}`, "application/json", 400},
		{`{"url":"https://valid.example","interval_seconds":300,"user_id":"other"}`, "application/json", 400},
		{`{"url":"https://duplicate.example","interval_seconds":300}`, "application/json", 409},
		{body, "text/plain", 400},
	} {
		w := call("PUT", m.ID, owner.AccessToken, "1", "", tc.body, tc.media, false)
		if w.Code != tc.status {
			t.Fatal(w.Code, w.Body.String())
		}
		unchanged(m)
	}
	w := call("PUT", m.ID, owner.AccessToken, "1", "https://client.example", body, "application/json", true)
	var updated monitor.Monitor
	if err = json.Unmarshal(w.Body.Bytes(), &updated); err != nil || w.Code != 200 || updated.ID != m.ID || updated.URL != "https://updated.example/path?q=1" || updated.IntervalSeconds != 90 || !updated.CreatedAt.Equal(old) || !updated.UpdatedAt.After(old) {
		t.Fatal(w.Code, w.Body.String(), err)
	}
	if w.Header().Get("Cache-Control") != "no-store" || strings.Contains(w.Body.String(), "user_id") {
		t.Fatal(w.Header(), w.Body.String())
	}
	updated.UserID = owner.User.ID
	unchanged(updated)
	// Saving the same values must still refresh updated_at.
	if _, err = pool.Exec(ctx, "UPDATE monitors SET updated_at=$2 WHERE id=$1", m.ID, old); err != nil {
		t.Fatal(err)
	}
	w = call("PUT", m.ID, owner.AccessToken, "1", "", body, "application/json", false)
	if err = json.Unmarshal(w.Body.Bytes(), &updated); err != nil || w.Code != 200 || !updated.CreatedAt.Equal(old) || !updated.UpdatedAt.After(old) {
		t.Fatal(w.Code, w.Body.String(), err)
	}
	w = call("DELETE", m.ID, owner.AccessToken, "1", "https://client.example", "", "", true)
	if w.Code != 204 || w.Body.Len() != 0 || w.Header().Get("Cache-Control") != "no-store" {
		t.Fatal(w.Code, w.Body.String())
	}
	items, err := s.List(ctx, owner.User.ID)
	if err != nil || len(items) != 1 || items[0].ID != duplicate.ID {
		t.Fatal(items, err)
	}
	for _, method := range []string{"PUT", "DELETE"} {
		w = call(method, m.ID, owner.AccessToken, "1", "", body, "application/json", false)
		if w.Code != 404 {
			t.Fatal(method, w.Code, w.Body.String())
		}
	}
	// Another user's identical URL does not conflict with an update.
	theirs, err := s.Create(ctx, other.User.ID, "https://shared.example", 300)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.Update(ctx, owner.User.ID, duplicate.ID, theirs.URL, 60); err != nil {
		t.Fatal(err)
	}
	if err = s.Delete(ctx, owner.User.ID, duplicate.ID); err != nil {
		t.Fatal(err)
	}
	items, err = s.List(ctx, owner.User.ID)
	if err != nil || len(items) != 0 {
		t.Fatal(items, err)
	}
	otherItems, err := s.List(ctx, other.User.ID)
	if err != nil || len(otherItems) != 1 || otherItems[0].ID != theirs.ID {
		t.Fatal(otherItems, err)
	}
	if _, err = pool.Exec(ctx, "DROP TABLE monitors"); err != nil {
		t.Fatal(err)
	}
	for _, method := range []string{"PUT", "DELETE"} {
		w = call(method, m.ID, owner.AccessToken, "1", "", body, "application/json", false)
		if w.Code != 500 || !strings.Contains(w.Body.String(), "internal_error") {
			t.Fatal(method, w.Code, w.Body.String())
		}
	}
}

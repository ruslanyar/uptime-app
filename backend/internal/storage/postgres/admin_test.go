package postgres_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"uptime-app/backend/internal/admin"
	"uptime-app/backend/internal/auth"
	"uptime-app/backend/internal/avatar"
	"uptime-app/backend/internal/config"
	"uptime-app/backend/internal/httpapi"
	"uptime-app/backend/internal/monitor"
	"uptime-app/backend/internal/storage/postgres"
)

func TestAdminUsers(t *testing.T) {
	ctx, pool, a, _ := setup(t)
	actor := register(t, ctx, a)
	store := postgres.New(pool)
	service := admin.NewService(store)
	if actor.User.Role != "user" {
		t.Fatal(actor.User)
	}
	for _, email := range []string{" User@EXAMPLE.com ", "user@example.com"} {
		if err := service.Grant(ctx, email); err != nil {
			t.Fatal(err)
		}
	}
	if err := service.Grant(ctx, "missing@example.com"); !errors.Is(err, admin.ErrNotFound) {
		t.Fatal(err)
	}
	if err := service.Grant(ctx, "broken"); !errors.Is(err, auth.ErrInvalid) {
		t.Fatal(err)
	}
	if user, err := a.Me(ctx, actor.AccessToken); err != nil || user.Role != "admin" {
		t.Fatal(user, err)
	}
	target, err := a.Register(ctx, "Other user", "other@example.com", "long password here")
	if err != nil {
		t.Fatal(err)
	}
	avatarDir := t.TempDir()
	avatarName := auth.NewID() + ".png"
	if err := os.WriteFile(filepath.Join(avatarDir, avatarName), []byte("avatar"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, "UPDATE users SET avatar_url=$1 WHERE id=$2", avatar.Prefix+avatarName, target.User.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := monitor.NewService(store).Create(ctx, target.User.ID, "https://example.com", 300); err != nil {
		t.Fatal(err)
	}
	for i := range 25 {
		if _, err := pool.Exec(ctx, "INSERT INTO users (id,name,email,password_hash) VALUES ($1,'Name',$2,'hash')", auth.NewID(), fmt.Sprintf("page-%02d@example.com", i)); err != nil {
			t.Fatal(err)
		}
	}
	h := httpapi.New(a, monitor.NewService(store), service, nil, config.Config{AvatarDir: avatarDir, AllowedOrigins: map[string]bool{"http://localhost:3000": true}})
	call := func(method, path, token, csrf, origin, bearer, body string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		if token != "" {
			r.AddCookie(&http.Cookie{Name: "access_token", Value: token})
		}
		if csrf != "" {
			r.Header.Set("X-CSRF-Protection", csrf)
		}
		if origin != "" {
			r.Header.Set("Origin", origin)
		}
		if bearer != "" {
			r.Header.Set("Authorization", bearer)
		}
		if body != "" {
			r.Header.Set("Content-Type", "application/json")
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w
	}
	get := func(page string) *httptest.ResponseRecorder {
		return call("GET", "/api/v1/admin/users"+page, actor.AccessToken, "", "", "", "")
	}
	for _, tc := range []struct {
		token, bearer string
		status        int
	}{{"", "", 401}, {target.AccessToken, "", 403}, {actor.AccessToken, "Bearer broken", 401}, {"", "Bearer " + actor.AccessToken, 200}} {
		w := call("GET", "/api/v1/admin/users", tc.token, "", "", tc.bearer, "")
		if w.Code != tc.status {
			t.Fatal(w.Code, w.Body.String())
		}
	}
	for _, tc := range []struct {
		query string
		count int
	}{{"", 25}, {"?page=2", 2}, {"?page=3", 0}, {"?page=2147483647", 0}} {
		w := get(tc.query)
		var result admin.List
		if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil || w.Code != 200 || len(result.Users) != tc.count || result.Total != 27 || result.PageSize != 25 {
			t.Fatal(w.Code, w.Body.String(), err)
		}
		if strings.Contains(w.Body.String(), "password_hash") || strings.Contains(w.Body.String(), "avatar_url") || w.Header().Get("Cache-Control") != "no-store" || strings.Contains(w.Body.String(), `"users":null`) {
			t.Fatal(w.Body.String())
		}
		for i := 1; i < len(result.Users); i++ {
			if result.Users[i-1].Email > result.Users[i].Email {
				t.Fatal("unsorted")
			}
		}
	}
	for _, query := range []string{"?page=0", "?page=-1", "?page=", "?page=a", "?page=1&page=2", "?page=2147483648"} {
		if w := get(query); w.Code != 400 {
			t.Fatal(query, w.Code)
		}
	}
	path := "/api/v1/admin/users/" + target.User.ID
	for _, tc := range []struct {
		token, csrf, origin, target string
		status                      int
	}{
		{"", "1", "", target.User.ID, 401}, {target.AccessToken, "1", "", actor.User.ID, 403},
		{actor.AccessToken, "", "", target.User.ID, 403}, {actor.AccessToken, "1", "https://evil.example", target.User.ID, 403},
		{actor.AccessToken, "1", "", "bad", 400}, {actor.AccessToken, "1", "", auth.NewID(), 404}, {actor.AccessToken, "1", "", actor.User.ID, 409},
	} {
		w := call("DELETE", "/api/v1/admin/users/"+tc.target, tc.token, tc.csrf, tc.origin, "", "")
		if w.Code != tc.status {
			t.Fatal(w.Code, w.Body.String(), tc.status)
		}
	}
	// Public writes must not allow role escalation, even with an admin's cookies.
	for _, tc := range []struct{ path, body string }{
		{"/api/v1/auth/register", `{"name":"New User","email":"new@example.com","password":"long password here","role":"admin"}`},
		{"/api/v1/auth/profile", `{"name":"Updated","role":"admin"}`},
	} {
		if w := call("POST", tc.path, actor.AccessToken, "1", "", "", tc.body); w.Code != 400 {
			t.Fatal(w.Code, w.Body.String())
		}
	}
	w := call("DELETE", "/api/v1/admin/users/"+strings.ToUpper(target.User.ID), actor.AccessToken, "1", "http://localhost:3000", "", "")
	if w.Code != 204 || w.Body.Len() != 0 {
		t.Fatal(w.Code, w.Body.String())
	}
	if _, err := os.Stat(filepath.Join(avatarDir, avatarName)); !os.IsNotExist(err) {
		t.Fatal("avatar remains", err)
	}
	for _, table := range []string{"users", "monitors", "sessions", "refresh_tokens"} {
		var count int
		query := "SELECT count(*) FROM " + table
		if table == "users" {
			query += " WHERE id='" + target.User.ID + "'"
		}
		if err := pool.QueryRow(ctx, query).Scan(&count); err != nil {
			t.Fatal(err)
		}
		want := 0
		if table == "sessions" || table == "refresh_tokens" {
			want = 1
		}
		if count != want {
			t.Fatal(table, count, want)
		}
	}
	if _, err := a.Me(ctx, target.AccessToken); !errors.Is(err, auth.ErrUnauthorized) {
		t.Fatal(err)
	}
	if _, err := a.Refresh(ctx, target.RefreshToken); !errors.Is(err, auth.ErrUnauthorized) {
		t.Fatal(err)
	}
	if w := call("GET", "/api/v1/monitors", target.AccessToken, "", "", "", ""); w.Code != 401 {
		t.Fatal(w.Code)
	}
	if w := call("DELETE", path, actor.AccessToken, "1", "", "", ""); w.Code != 404 {
		t.Fatal(w.Code)
	}
	// A stale JWT must not retain admin access after a role change.
	if _, err := pool.Exec(ctx, "UPDATE users SET role='user' WHERE id=$1", actor.User.ID); err != nil {
		t.Fatal(err)
	}
	if w := get(""); w.Code != 403 {
		t.Fatal(w.Code)
	}
}

func TestAdminDeleteConcurrentGrant(t *testing.T) {
	ctx, pool, a, _ := setup(t)
	actor := register(t, ctx, a)
	store := postgres.New(pool)
	if err := store.GrantAdmin(ctx, actor.User.Email); err != nil {
		t.Fatal(err)
	}
	target, err := a.Register(ctx, "Other", "concurrent@example.com", "long password here")
	if err != nil {
		t.Fatal(err)
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, "UPDATE users SET role='admin' WHERE id=$1", target.User.ID); err != nil {
		t.Fatal(err)
	}
	result := make(chan error, 1)
	go func() { _, err := admin.NewService(store).Delete(ctx, actor.User.ID, target.User.ID); result <- err }()
	// Wait for the actual row-lock contention rather than depending on scheduler timing.
	deadline := time.Now().Add(5 * time.Second)
	for {
		var waiting bool
		if err := pool.QueryRow(ctx, "SELECT EXISTS (SELECT 1 FROM pg_stat_activity WHERE datname=current_database() AND wait_event_type='Lock' AND query LIKE '%FOR UPDATE%')").Scan(&waiting); err != nil {
			t.Fatal(err)
		}
		if waiting {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("delete did not wait for the grant")
		}
		time.Sleep(10 * time.Millisecond)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	if err := <-result; !errors.Is(err, admin.ErrProtected) {
		t.Fatal(err)
	}
	if user, err := a.Me(ctx, target.AccessToken); err != nil || user.Role != "admin" {
		t.Fatal(user, err)
	}
	if _, err := admin.NewService(store).Delete(ctx, actor.User.ID, target.User.ID); !errors.Is(err, admin.ErrProtected) {
		t.Fatal(err)
	}
}

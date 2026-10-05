package postgres_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"uptime-app/backend/internal/auth"
	"uptime-app/backend/internal/config"
	"uptime-app/backend/internal/httpapi"
)

func cookieNamed(t *testing.T, w *httptest.ResponseRecorder, name string) *http.Cookie {
	t.Helper()
	for _, c := range w.Result().Cookies() {
		if c.Name == name {
			return c
		}
	}
	t.Fatal("missing cookie", name)
	return nil
}
func postAuth(handler http.Handler, path, body string, cookie *http.Cookie) *httptest.ResponseRecorder {
	r := httptest.NewRequest("POST", "/api/v1/auth/"+path, strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("X-CSRF-Protection", "1")
	r.Header.Set("Origin", "http://localhost:3000")
	if cookie != nil {
		r.AddCookie(cookie)
	}
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)
	return w
}
func TestHTTPContract(t *testing.T) {
	_, _, service, _ := setup(t)
	handler := httpapi.New(service, config.Config{AllowedOrigins: map[string]bool{"http://localhost:3000": true}, CookieSameSite: http.SameSiteLaxMode})
	post := func(path, body string, cookie *http.Cookie) *httptest.ResponseRecorder {
		return postAuth(handler, path, body, cookie)
	}
	registration := post("register", `{"name":"User","email":"user@example.com","password":"long password here"}`, nil)
	if registration.Code != 201 {
		t.Fatal(registration.Code)
	}
	var result struct {
		User auth.User `json:"user"`
	}
	if e := json.Unmarshal(registration.Body.Bytes(), &result); e != nil {
		t.Fatal(e)
	}
	var payload map[string]any
	_ = json.Unmarshal(registration.Body.Bytes(), &payload)
	if len(payload) != 1 {
		t.Fatal("unexpected public fields")
	}
	access := cookieNamed(t, registration, "access_token")
	refresh := cookieNamed(t, registration, "refresh_token")
	me := func(cookie *http.Cookie, authorization *string) *httptest.ResponseRecorder {
		r := httptest.NewRequest("GET", "/api/v1/auth/me", nil)
		if cookie != nil {
			r.AddCookie(cookie)
		}
		if authorization != nil {
			r.Header.Set("Authorization", *authorization)
		}
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		return w
	}
	if w := me(access, nil); w.Code != 200 || !strings.Contains(w.Body.String(), result.User.ID) {
		t.Fatal("cookie me", w.Code)
	}
	bearer := "Bearer " + access.Value
	if w := me(nil, &bearer); w.Code != 200 {
		t.Fatal("bearer me", w.Code)
	}
	invalid := "Bearer broken"
	if w := me(access, &invalid); w.Code != 401 {
		t.Fatal("invalid bearer fell back to cookie")
	}
	if w := me(nil, nil); w.Code != 401 {
		t.Fatal("missing access accepted")
	}
	for _, raw := range []string{"broken", expiredAccess(t, result.User.ID)} {
		if w := me(&http.Cookie{Name: "access_token", Value: raw}, nil); w.Code != 401 {
			t.Fatal("invalid cookie accepted")
		}
	}
	conflict := post("register", `{"name":"User","email":"USER@example.com","password":"long password here"}`, nil)
	if conflict.Code != 409 || len(conflict.Result().Cookies()) != 0 {
		t.Fatal(conflict.Code)
	}
	unknown := post("login", `{"email":"unknown@example.com","password":"long password here"}`, nil)
	wrong := post("login", `{"email":"user@example.com","password":"wrong password here"}`, nil)
	if unknown.Code != 401 || wrong.Code != 401 || unknown.Body.String() != wrong.Body.String() {
		t.Fatal("login errors differ")
	}
	for _, body := range []string{`{"email":"invalid","password":"long password here"}`, `{"email":"user@example.com","password":"short"}`} {
		if w := post("login", body, nil); w.Code != 400 {
			t.Fatal("invalid login", w.Code)
		}
	}
	login := post("login", `{"email":"user@example.com","password":"long password here"}`, nil)
	if login.Code != 200 {
		t.Fatal(login.Code)
	}
	rotated := post("refresh", "", refresh)
	if rotated.Code != 200 {
		t.Fatal(rotated.Code)
	}
	newAccess := cookieNamed(t, rotated, "access_token")
	newRefresh := cookieNamed(t, rotated, "refresh_token")
	if newAccess.Value == access.Value || newRefresh.Value == refresh.Value || !newRefresh.Expires.Equal(refresh.Expires) {
		t.Fatal("rotation did not replace both tokens or extended session")
	}
	if w := me(newAccess, nil); w.Code != 200 {
		t.Fatal("rotated access invalid")
	}
	logout := post("logout", "", newRefresh)
	if logout.Code != 204 || len(logout.Result().Cookies()) != 2 {
		t.Fatal("logout", logout.Code)
	}
	if w := me(nil, &bearer); w.Code != 200 {
		t.Fatal("JWT revoked prematurely")
	}
	denied := post("refresh", "", newRefresh)
	if denied.Code != 401 || len(denied.Result().Cookies()) != 2 {
		t.Fatal("revoked refresh", denied.Code)
	}
	for _, c := range denied.Result().Cookies() {
		if c.MaxAge != -1 || c.Value != "" {
			t.Fatal("401 did not clear cookies")
		}
	}
	if w := post("logout", "", nil); w.Code != 204 || len(w.Result().Cookies()) != 2 {
		t.Fatal("logout without session")
	}
	if w := post("refresh", "", cookieNamed(t, login, "refresh_token")); w.Code != 200 {
		t.Fatal("independent login", w.Code)
	}
}
func expiredAccess(t *testing.T, id string) string {
	t.Helper()
	raw, e := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.RegisteredClaims{Subject: id, Issuer: "issuer", Audience: jwt.ClaimStrings{"audience"}, ID: auth.NewID(), IssuedAt: jwt.NewNumericDate(time.Now().Add(-2 * time.Hour)), ExpiresAt: jwt.NewNumericDate(time.Now().Add(-time.Hour))}).SignedString([]byte(strings.Repeat("s", 32)))
	if e != nil {
		t.Fatal(e)
	}
	return raw
}
func TestHTTPCookiesOnCommitFailure(t *testing.T) {
	ctx, pool, service, _ := setup(t)
	_, e := pool.Exec(ctx, `CREATE FUNCTION fail_commit() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'forced commit failure'; END $$; CREATE CONSTRAINT TRIGGER reject_token AFTER INSERT ON refresh_tokens DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION fail_commit();`)
	if e != nil {
		t.Fatal(e)
	}
	handler := httpapi.New(service, config.Config{AllowedOrigins: map[string]bool{"http://localhost:3000": true}})
	w := postAuth(handler, "register", `{"name":"User","email":"u@example.com","password":"long password here"}`, nil)
	if w.Code != 500 || len(w.Result().Cookies()) != 0 {
		t.Fatal("commit failure issued cookies")
	}
	var n int
	if e = pool.QueryRow(ctx, "SELECT count(*) FROM users").Scan(&n); e != nil || n != 0 {
		t.Fatal("registration not rolled back", e)
	}
}

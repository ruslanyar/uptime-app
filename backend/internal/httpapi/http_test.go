package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
	"uptime-app/backend/internal/auth"
	"uptime-app/backend/internal/config"
)

type fakeService struct {
	calls   int
	meToken string
	err     error
}

func (f *fakeService) result() (auth.Result, error) {
	f.calls++
	return auth.Result{AccessToken: "access", TokenType: "Bearer", ExpiresIn: 86400, User: auth.User{ID: auth.NewID(), Name: "User", Email: "user@example.com"}, RefreshToken: "secret-refresh", SessionExpires: time.Now().Add(time.Hour)}, f.err
}
func (f *fakeService) Register(context.Context, string, string, string) (auth.Result, error) {
	return f.result()
}
func (f *fakeService) Login(context.Context, string, string) (auth.Result, error) { return f.result() }
func (f *fakeService) Refresh(context.Context, string) (auth.Result, error)       { return f.result() }
func (f *fakeService) Logout(context.Context, string) error                       { f.calls++; return f.err }
func (f *fakeService) Me(_ context.Context, token string) (auth.User, error) {
	f.calls++
	f.meToken = token
	return auth.User{Name: "User"}, f.err
}
func cfg() config.Config {
	return config.Config{AllowedOrigins: map[string]bool{"http://localhost:3000": true, "https://client.example": true}, CookieSecure: true, CookieSameSite: http.SameSiteNoneMode}
}
func request(h http.Handler, method, path, origin, csrf, body string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, "/api/v1/auth/"+path, strings.NewReader(body))
	if origin != "" {
		r.Header.Set("Origin", origin)
	}
	if csrf != "" {
		r.Header.Set("X-CSRF-Protection", csrf)
	}
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Sec-Fetch-Site", "cross-site")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}
func TestProtection(t *testing.T) {
	for _, path := range []string{"register", "login", "refresh", "logout", "profile"} {
		for _, origin := range []string{"", "http://localhost:3000", "https://client.example", "http://localhost:3001", "https://localhost:3000", "https://unknown.example", "null"} {
			for _, csrf := range []string{"", "0", "1"} {
				f := &fakeService{}
				w := request(New(f, cfg()), "POST", path, origin, csrf, `{"email":"e@x.com","password":"password"}`)
				allowed := (origin == "" || cfg().AllowedOrigins[origin]) && csrf == "1"
				if !allowed && (w.Code != 403 || f.calls != 0 || len(w.Result().Cookies()) != 0) {
					t.Fatalf("%s %s %s: %d calls %d", path, origin, csrf, w.Code, f.calls)
				}
				if origin != "" && cfg().AllowedOrigins[origin] && (w.Header().Get("Access-Control-Allow-Origin") != origin || w.Header().Get("Access-Control-Allow-Credentials") != "true") {
					t.Fatal("cors missing")
				}
			}
		}
	}
}
func TestPreflight(t *testing.T) {
	for _, tc := range []struct {
		origin, method, headers string
		status                  int
	}{{"http://localhost:3000", "POST", "content-type, X-CSRF-Protection", 204}, {"https://client.example", "GET", "Authorization", 204}, {"null", "POST", "", 403}, {"", "POST", "", 403}, {"https://client.example", "DELETE", "", 403}, {"https://client.example", "POST", "X-Unknown", 403}} {
		f := &fakeService{}
		r := httptest.NewRequest("OPTIONS", "/api/v1/auth/refresh", nil)
		r.Header.Set("Origin", tc.origin)
		r.Header.Set("Access-Control-Request-Method", tc.method)
		r.Header.Set("Access-Control-Request-Headers", tc.headers)
		w := httptest.NewRecorder()
		New(f, cfg()).ServeHTTP(w, r)
		if w.Code != tc.status || f.calls != 0 {
			t.Fatal(tc, w.Code, f.calls)
		}
	}
}
func TestResponses(t *testing.T) {
	for _, path := range []string{"register", "login", "refresh"} {
		f := &fakeService{}
		body := `{"email":"u@x.com","password":"long password here"}`
		if path == "register" {
			body = `{"name":"User","email":"u@x.com","password":"long password here"}`
		}
		w := request(New(f, cfg()), "POST", path, "https://client.example", "1", body)
		want := 200
		if path == "register" {
			want = 201
		}
		if w.Code != want {
			t.Fatal(path, w.Code)
		}
		var payload map[string]json.RawMessage
		if err := json.Unmarshal(w.Body.Bytes(), &payload); err != nil || len(payload) != 1 || payload["user"] == nil {
			t.Fatal("unexpected public response", w.Body.String())
		}
		var user map[string]any
		_ = json.Unmarshal(payload["user"], &user)
		if len(user) != 3 || user["id"] == nil || user["name"] != "User" || user["email"] != "user@example.com" {
			t.Fatal("unexpected user fields")
		}
		if w.Header().Get("Cache-Control") != "no-store" {
			t.Fatal("cache enabled")
		}
		cookies := w.Result().Cookies()
		if len(cookies) != 2 {
			t.Fatal("expected two cookies")
		}
		for _, c := range cookies {
			if !c.HttpOnly || !c.Secure || c.SameSite != http.SameSiteNoneMode || c.Domain != "" {
				t.Fatal("cookie attributes")
			}
			switch c.Name {
			case "access_token":
				if c.Value != "access" || c.Path != "/api/v1" || c.MaxAge < 86390 || c.MaxAge > 86400 || time.Until(c.Expires) < auth.AccessTTL-time.Second*2 {
					t.Fatal("access cookie lifetime or path")
				}
			case "refresh_token":
				if c.Value != "secret-refresh" || c.Path != "/api/v1/auth" || c.MaxAge < 3590 || c.MaxAge > 3600 || time.Until(c.Expires) > time.Hour {
					t.Fatal("refresh cookie lifetime or path")
				}
			default:
				t.Fatal("unexpected cookie")
			}
		}
	}
	for _, mode := range []http.SameSite{http.SameSiteLaxMode, http.SameSiteStrictMode} {
		local := cfg()
		local.CookieSecure = false
		local.CookieSameSite = mode
		w := request(New(&fakeService{}, local), "POST", "refresh", "", "1", "")
		for _, c := range w.Result().Cookies() {
			if c.Secure || c.SameSite != mode {
				t.Fatal("local cookie attributes")
			}
		}
		w = request(New(&fakeService{}, local), "POST", "logout", "", "1", "")
		if w.Code != 204 {
			t.Fatal(w.Code)
		}
		assertDeletedCookies(t, w, local)
	}
	for _, tc := range []struct {
		err    error
		status int
	}{{auth.ErrInvalid, 400}, {auth.ErrUnauthorized, 401}, {auth.ErrConflict, 409}, {errors.New("commit failed"), 500}} {
		for _, path := range []string{"register", "login", "refresh"} {
			body := `{"email":"u@x.com","password":"long password here"}`
			if path == "register" {
				body = `{"name":"User","email":"u@x.com","password":"long password here"}`
			}
			w := request(New(&fakeService{err: tc.err}, cfg()), "POST", path, "https://client.example", "1", body)
			if w.Code != tc.status || !strings.Contains(w.Body.String(), `"error"`) || w.Header().Get("Access-Control-Allow-Origin") == "" {
				t.Fatal(path, w.Code)
			}
			if path == "refresh" && tc.status == 401 {
				assertDeletedCookies(t, w, cfg())
			} else if len(w.Result().Cookies()) != 0 {
				t.Fatal("cookies changed on failed request")
			}
		}
	}
	w := request(New(&fakeService{err: errors.New("database unavailable")}, cfg()), "POST", "logout", "", "1", "")
	if w.Code != 500 || len(w.Result().Cookies()) != 0 {
		t.Fatal("failed logout changed cookies")
	}
}
func assertDeletedCookies(t *testing.T, w *httptest.ResponseRecorder, cfg config.Config) {
	t.Helper()
	cookies := w.Result().Cookies()
	if len(cookies) != 2 {
		t.Fatal("expected two deleted cookies")
	}
	for _, c := range cookies {
		path := "/api/v1"
		if c.Name == "refresh_token" {
			path = "/api/v1/auth"
		} else if c.Name != "access_token" {
			t.Fatal("unexpected cookie")
		}
		if c.Value != "" || c.MaxAge != -1 || !c.Expires.Before(time.Now()) || c.Path != path || !c.HttpOnly || c.Domain != "" || c.Secure != cfg.CookieSecure || c.SameSite != cfg.CookieSameSite {
			t.Fatal("incorrect cookie deletion")
		}
	}
}
func TestMeTokenSelection(t *testing.T) {
	for _, tc := range []struct {
		headers       []string
		cookie, token string
		status        int
	}{
		{nil, "cookie-token", "cookie-token", 200},
		{[]string{"Bearer header-token"}, "cookie-token", "header-token", 200},
		{[]string{"Bearer header-token"}, "", "header-token", 200},
		{[]string{""}, "cookie-token", "", 401},
		{[]string{"Basic invalid"}, "cookie-token", "", 401},
		{[]string{"Bearer"}, "cookie-token", "", 401},
		{[]string{"Bearer one", "Bearer two"}, "cookie-token", "", 401},
		{nil, "", "", 401},
	} {
		f := &fakeService{}
		r := httptest.NewRequest("GET", "/api/v1/auth/me", nil)
		if tc.headers != nil {
			r.Header["Authorization"] = tc.headers
		}
		if tc.cookie != "" {
			r.AddCookie(&http.Cookie{Name: "access_token", Value: tc.cookie})
		}
		w := httptest.NewRecorder()
		New(f, cfg()).ServeHTTP(w, r)
		if w.Code != tc.status || f.meToken != tc.token || w.Header().Get("Cache-Control") != "no-store" {
			t.Fatal("me token selection", w.Code, f.meToken)
		}
		if tc.status == 401 && f.calls != 0 {
			t.Fatal("malformed auth reached service")
		}
	}
}
func TestJSONAndMe(t *testing.T) {
	for _, body := range []string{"", `null`, `{`, `{}`, `{"unknown":1}`, `{} {}`, strings.Repeat("x", 20000)} {
		f := &fakeService{}
		w := request(New(f, cfg()), "POST", "login", "", "1", body)

		if w.Code != 400 || f.calls != 0 {
			t.Fatal(body[:min(len(body), 30)], w.Code)
		}
	}
	f := &fakeService{}
	h := New(f, cfg())
	w := request(h, "GET", "me", "", "", "")
	if w.Code != 401 || f.calls != 0 {
		t.Fatal(w.Code)
	}
	r := httptest.NewRequest("GET", "/api/v1/auth/me", nil)
	r.Header.Set("Authorization", "Bearer access")
	w = httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 200 || f.calls != 1 {
		t.Fatal(w.Code)
	}
	r = httptest.NewRequest("POST", "/api/v1/auth/login", strings.NewReader(`{}`))
	r.Header.Set("X-CSRF-Protection", "1")
	r.Header.Set("Content-Type", "text/plain")
	w = httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 400 {
		t.Fatal(w.Code)
	}
}

func (f *fakeService) UpdateProfile(_ context.Context, token, name string, avatarURL ...string) (auth.User, error) {
	f.calls++
	f.meToken = token
	return auth.User{ID: "id", Name: name, Email: "user@example.com"}, f.err
}
func TestUpdateProfile(t *testing.T) {
	for _, tc := range []struct {
		body, token string
		status      int
	}{
		{`{"name":"New"}`, "access", 200},
		{`{"name":"New"}`, "", 401},
		{`{"name":"New","email":"other@example.com"}`, "access", 400},
		{`null`, "access", 400},
	} {
		f := &fakeService{}
		r := httptest.NewRequest("POST", "/api/v1/auth/profile", strings.NewReader(tc.body))
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("X-CSRF-Protection", "1")
		if tc.token != "" {
			r.AddCookie(&http.Cookie{Name: "access_token", Value: tc.token})
		}
		w := httptest.NewRecorder()
		New(f, cfg()).ServeHTTP(w, r)
		if w.Code != tc.status {
			t.Fatal(w.Code, w.Body.String())
		}
		if tc.status == 200 && (f.meToken != tc.token || !strings.Contains(w.Body.String(), `"name":"New"`) || len(w.Result().Cookies()) != 0) {
			t.Fatal(w.Body.String())
		}
	}
}

package httpapi

import (
	"context"
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
	calls int
	err   error
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
func (f *fakeService) Me(context.Context, string) (auth.User, error) {
	f.calls++
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
	for _, path := range []string{"register", "login", "refresh", "logout"} {
		for _, origin := range []string{"", "http://localhost:3000", "https://client.example", "http://localhost:3001", "https://localhost:3000", "https://unknown.example", "null"} {
			for _, csrf := range []string{"", "0", "1"} {
				f := &fakeService{}
				w := request(New(f, cfg()), "POST", path, origin, csrf, `{"email":"e@x.com","password":"password"}`)
				allowed := (origin == "" || cfg().AllowedOrigins[origin]) && csrf == "1"
				if !allowed && (w.Code != 403 || f.calls != 0) {
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
		w := request(New(f, cfg()), "POST", path, "https://client.example", "1", `{"name":"User","email":"u@x.com","password":"long password here"}`)
		if path == "login" {
			w = request(New(f, cfg()), "POST", path, "https://client.example", "1", `{"email":"u@x.com","password":"long password here"}`)
		}
		want := 200
		if path == "register" {
			want = 201
		}
		if w.Code != want {
			t.Fatal(path, w.Code, w.Body.String())
		}
		if strings.Contains(w.Body.String(), "secret-refresh") || strings.Contains(w.Body.String(), "password") || w.Header().Get("Cache-Control") != "no-store" {
			t.Fatal(w.Body.String())
		}
		cookies := w.Result().Cookies()
		if len(cookies) != 1 {
			t.Fatal("cookie missing")
		}
		c := cookies[0]
		if !c.HttpOnly || !c.Secure || c.SameSite != http.SameSiteNoneMode || c.Path != "/api/v1/auth" || c.Domain != "" || c.MaxAge > 3600 || c.MaxAge < 3590 {
			t.Fatal(c)
		}
	}
	f := &fakeService{}
	local := cfg()
	local.CookieSecure = false
	local.CookieSameSite = http.SameSiteLaxMode
	w := request(New(f, local), "POST", "refresh", "", "1", "")
	c := w.Result().Cookies()[0]
	if c.Secure || c.SameSite != http.SameSiteLaxMode {
		t.Fatal(c)
	}
	w = request(New(f, local), "POST", "logout", "", "1", "")
	c = w.Result().Cookies()[0]
	if w.Code != 204 || c.MaxAge != -1 || c.Path != "/api/v1/auth" || c.SameSite != http.SameSiteLaxMode {
		t.Fatal(w.Code, c)
	}
	for _, tc := range []struct {
		err    error
		status int
	}{{auth.ErrInvalid, 400}, {auth.ErrUnauthorized, 401}, {auth.ErrConflict, 409}, {errors.New("commit failed"), 500}} {
		f := &fakeService{err: tc.err}
		w := request(New(f, cfg()), "POST", "refresh", "https://client.example", "1", "")
		if w.Code != tc.status || len(w.Result().Cookies()) != 0 || !strings.Contains(w.Body.String(), `"error"`) || w.Header().Get("Access-Control-Allow-Origin") == "" {
			t.Fatal(w.Code, w.Body.String())
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

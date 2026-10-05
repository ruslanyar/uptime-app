package postgres_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"uptime-app/backend/internal/auth"
	"uptime-app/backend/internal/config"
	"uptime-app/backend/internal/httpapi"
)

func TestHTTPContract(t *testing.T) {
	_, _, service, _ := setup(t)
	handler := httpapi.New(service, config.Config{AllowedOrigins: map[string]bool{"http://localhost:3000": true}, CookieSameSite: http.SameSiteLaxMode})
	post := func(path, body string, cookie *http.Cookie) *httptest.ResponseRecorder {
		t.Helper()
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
	registration := post("register", `{"name":"User","email":"user@example.com","password":"long password here"}`, nil)
	if registration.Code != 201 {
		t.Fatal(registration.Code, registration.Body.String())
	}
	var result auth.Result
	if e := json.Unmarshal(registration.Body.Bytes(), &result); e != nil {
		t.Fatal(e)
	}
	cookie := registration.Result().Cookies()[0]
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
			t.Fatal("invalid login input", w.Code)
		}
	}
	login := post("login", `{"email":"user@example.com","password":"long password here"}`, nil)
	if login.Code != 200 {
		t.Fatal(login.Code)
	}
	refreshed := post("refresh", "", cookie)
	if refreshed.Code != 200 {
		t.Fatal(refreshed.Code)
	}
	r := httptest.NewRequest("GET", "/api/v1/auth/me", nil)
	r.Header.Set("Authorization", "Bearer "+result.AccessToken)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)
	if w.Code != 200 || !strings.Contains(w.Body.String(), result.User.ID) {
		t.Fatal(w.Code, w.Body.String())
	}
	r.Header.Set("Authorization", "Bearer broken")
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, r)
	if w.Code != 401 {
		t.Fatal(w.Code)
	}
	if w = post("logout", "", refreshed.Result().Cookies()[0]); w.Code != 204 {
		t.Fatal(w.Code)
	}
	if w = post("refresh", "", refreshed.Result().Cookies()[0]); w.Code != 401 {
		t.Fatal(w.Code)
	}
	if w = post("logout", "", nil); w.Code != 204 {
		t.Fatal(w.Code)
	}
	if w = post("refresh", "", login.Result().Cookies()[0]); w.Code != 200 {
		t.Fatal("independent login", w.Code)
	}
}

package monitorcontroller

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"uptime-app/backend/internal/auth"
	"uptime-app/backend/internal/favicon"
)

type fakeIcons struct {
	calls int
	err   error
}

func (f *fakeIcons) Resolve(context.Context, string) (favicon.Icon, error) {
	f.calls++
	return favicon.Icon{Data: []byte("icon"), Type: "image/png"}, f.err
}

func TestFaviconHTTP(t *testing.T) {
	for _, tc := range []struct {
		name, token, query string
		authErr, iconErr   error
		status             int
	}{
		{name: "found", token: "access", query: "https://example.com", status: 200},
		{name: "missing", token: "access", query: "https://example.com", iconErr: errors.New("private detail"), status: 204},
		{name: "unauthenticated", query: "https://example.com", status: 401},
		{name: "expired", token: "access", query: "https://example.com", authErr: auth.ErrUnauthorized, status: 401},
		{name: "invalid URL", token: "access", query: "file:///etc/passwd", status: 400},
		{name: "empty URL", token: "access", status: 400},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest("GET", "/api/v1/monitors/favicon?url="+tc.query, nil)
			if tc.token != "" {
				r.AddCookie(&http.Cookie{Name: "access_token", Value: tc.token})
			}
			w := httptest.NewRecorder()
			c := New(&fakeAuth{err: tc.authErr}, &fakeMonitor{})
			icons := &fakeIcons{err: tc.iconErr}
			c.icons = icons
			c.Favicon(w, r)
			if w.Code != tc.status || w.Header().Get("Cache-Control") != "no-store" {
				t.Fatal(w.Code, w.Body.String())
			}
			if tc.status == 200 && (w.Body.String() != "icon" || w.Header().Get("Content-Type") != "image/png" || w.Header().Get("Content-Security-Policy") == "") {
				t.Fatal(w)
			}
			if tc.status == 204 && w.Body.Len() != 0 {
				t.Fatal(w.Body.String())
			}
			if tc.status >= 400 && icons.calls != 0 {
				t.Fatal("invalid request fetched a site")
			}
		})
	}
}

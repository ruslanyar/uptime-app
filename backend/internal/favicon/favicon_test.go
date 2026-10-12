package favicon

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"net/url"
	"strings"
	"testing"
)

const svg = `<svg xmlns="http://www.w3.org/2000/svg"><circle r="8"/></svg>`

func TestDiscovery(t *testing.T) {
	for _, tc := range []struct {
		name, page, iconPath string
	}{
		{"relative", `<link rel="icon" href="assets/logo.svg">`, "/nested/assets/logo.svg"},
		{"shortcut and entities", `<link href='/logo.svg?a=1&amp;b=2' rel='SHORTCUT ICON'>`, "/logo.svg?a=1&b=2"},
		{"base", `<base href="/static/"><link rel="icon" href="logo.svg">`, "/static/logo.svg"},
		{"apple", `<link rel="apple-touch-icon" href="/apple.svg">`, "/apple.svg"},
		{"fallback", `<link rel="icon" href="/missing.svg">`, "/favicon.ico"},
		{"invalid image fallback", `<link rel="icon" href="/html">`, "/favicon.ico"},
		{"no links", `<html><head></head></html>`, "/favicon.ico"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.RequestURI() {
				case "/nested/page":
					_, _ = w.Write([]byte(tc.page))
				case tc.iconPath:
					_, _ = w.Write([]byte(svg))
				case "/html":
					w.Header().Set("Content-Type", "image/png")
					_, _ = w.Write([]byte("<html>Not an image</html>"))
				default:
					w.WriteHeader(404)
				}
			}))
			defer server.Close()
			r := &Resolver{client: server.Client()}
			icon, err := r.Resolve(context.Background(), server.URL+"/nested/page")
			if err != nil || icon.Type != "image/svg+xml" || string(icon.Data) != svg {
				t.Fatal(icon, err)
			}
		})
	}
}

func TestRedirectAndCrossOrigin(t *testing.T) {
	cdn := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(svg))
	}))
	defer cdn.Close()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/old" {
			http.Redirect(w, r, "/new/page", http.StatusFound)
			return
		}
		_, _ = w.Write([]byte(`<link rel="icon" href="` + cdn.URL + `/icon.svg">`))
	}))
	defer server.Close()
	icon, err := (&Resolver{client: server.Client()}).Resolve(context.Background(), server.URL+"/old")
	if err != nil || string(icon.Data) != svg {
		t.Fatal(icon, err)
	}
}

func TestUnavailable(t *testing.T) {
	for _, body := range []string{"", "<html>missing</html>", strings.Repeat("x", maxBody+1)} {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(body))
		}))
		_, err := (&Resolver{client: server.Client()}).Resolve(context.Background(), server.URL)
		server.Close()
		if !errors.Is(err, errUnavailable) {
			t.Fatal(err)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := NewResolver().Resolve(ctx, "https://example.com"); err == nil {
		t.Fatal("cancelled request succeeded")
	}
}

func TestFallbackAfterRedirectWithUnreadableHTML(t *testing.T) {
	for _, tc := range []struct {
		name   string
		status int
		body   string
	}{
		{"oversized", http.StatusOK, strings.Repeat("x", maxBody+1)},
		{"empty", http.StatusOK, ""},
		{"forbidden", http.StatusForbidden, "blocked page"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			final := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/favicon.ico" {
					_, _ = w.Write([]byte(svg))
					return
				}
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.body))
			}))
			defer final.Close()
			original := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/favicon.ico" {
					t.Error("requested fallback from the original origin")
					w.WriteHeader(http.StatusNotFound)
					return
				}
				http.Redirect(w, r, final.URL, http.StatusFound)
			}))
			defer original.Close()
			icon, err := (&Resolver{client: original.Client()}).Resolve(context.Background(), original.URL)
			if err != nil || string(icon.Data) != svg {
				t.Fatal(icon, err)
			}
		})
	}
}

func TestPublicDestinations(t *testing.T) {
	for _, ip := range []string{"127.0.0.1", "10.0.0.1", "172.16.1.2", "192.168.1.1", "169.254.169.254", "100.64.0.1", "0.0.0.0", "192.0.2.1", "198.18.0.1", "224.0.0.1", "240.1.1.1", "::1", "::ffff:127.0.0.1", "fc00::1", "fe80::1", "2001:db8::1", "2002:7f00:1::", "64:ff9b::7f00:1"} {
		if publicIP(netip.MustParseAddr(ip)) {
			t.Errorf("allowed %s", ip)
		}
	}
	for _, ip := range []string{"1.1.1.1", "8.8.8.8", "2606:4700:4700::1111", "::ffff:8.8.8.8"} {
		if !publicIP(netip.MustParseAddr(ip)) {
			t.Errorf("blocked %s", ip)
		}
	}
	for _, raw := range []string{"http://127.0.0.1", "http://[::1]", "http://localhost", "file:///etc/passwd", "https://user:secret@example.com"} {
		if _, err := NewResolver().Resolve(context.Background(), raw); err == nil {
			t.Errorf("fetched %s", raw)
		}
	}
}

func TestCandidateLimitAndSchemes(t *testing.T) {
	page, _ := url.Parse("https://example.com/path")
	links := candidates([]byte(`<link rel="icon" href="file:///etc/passwd"><link rel="icon" href="data:image/svg+xml,test">`+strings.Repeat(`<link rel="icon" href="/icon.svg">`, 20)), page)
	if len(links) != 4 {
		t.Fatal(len(links))
	}
}

func TestRedirectGuard(t *testing.T) {
	r := NewResolver()
	for _, raw := range []string{"file:///etc/passwd", "https://user:pass@example.com"} {
		u, _ := url.Parse(raw)
		if r.client.CheckRedirect(&http.Request{URL: u}, nil) == nil {
			t.Fatal("unsafe redirect allowed")
		}
	}
	u, _ := url.Parse("https://example.com")
	if r.client.CheckRedirect(&http.Request{URL: u}, make([]*http.Request, 3)) == nil {
		t.Fatal("unbounded redirects")
	}
	if _, err := publicDial(context.Background(), "tcp", "127.0.0.1:80"); err == nil {
		t.Fatal("private connection allowed")
	}
}

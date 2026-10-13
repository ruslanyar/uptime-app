package favicon

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"net/url"
	"strings"
	"testing"
	"testing/synctest"
	"time"
	"uptime-app/backend/internal/publichttp"
)

const svg = `<svg xmlns="http://www.w3.org/2000/svg"><circle r="8"/></svg>`

func TestResolveNetworkBudget(t *testing.T) {
	for _, tc := range []struct {
		name           string
		slowDNS        bool
		dnsDelay       time.Duration
		callerDeadline time.Duration
		cancelRunning  bool
		wantDuration   time.Duration
		wantAttempts   int
	}{
		{name: "slow DNS", slowDNS: true, wantDuration: 5 * time.Second},
		{name: "caller deadline during DNS", slowDNS: true, callerDeadline: time.Second, wantDuration: time.Second},
		{name: "unavailable IPs", wantDuration: 5 * time.Second, wantAttempts: 3},
		{name: "DNS consumes connection budget", dnsDelay: 4 * time.Second, wantDuration: 5 * time.Second, wantAttempts: 1},
		{name: "cancel running DNS", slowDNS: true, cancelRunning: true, wantDuration: 500 * time.Millisecond},
		{name: "cancel running connection", cancelRunning: true, wantDuration: 500 * time.Millisecond, wantAttempts: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				start := time.Now()
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				budget := 5 * time.Second
				if tc.callerDeadline != 0 {
					var cancelDeadline context.CancelFunc
					ctx, cancelDeadline = context.WithTimeout(ctx, tc.callerDeadline)
					defer cancelDeadline()
					budget = tc.callerDeadline
				}
				started := make(chan struct{}, 1)
				if tc.cancelRunning {
					go func() {
						<-started
						time.Sleep(tc.wantDuration)
						cancel()
					}()
				}
				lookupCalls, lookupFinished, attempts, connectionsFinished := 0, 0, 0, 0
				dialer := publichttp.Dialer{
					Lookup: func(ctx context.Context, network, host string) ([]netip.Addr, error) {
						lookupCalls++
						defer func() { lookupFinished++ }()
						if deadline, ok := ctx.Deadline(); !ok || !deadline.Equal(start.Add(budget)) {
							t.Errorf("DNS deadline = %v, %v; want %v", deadline, ok, start.Add(budget))
						}
						if tc.slowDNS {
							started <- struct{}{}
							<-ctx.Done()
							return nil, ctx.Err()
						}
						time.Sleep(tc.dnsDelay)
						return []netip.Addr{netip.MustParseAddr("1.1.1.1"), netip.MustParseAddr("8.8.8.8"), netip.MustParseAddr("9.9.9.9"), netip.MustParseAddr("8.8.4.4")}, nil
					},
					Connect: func(ctx context.Context, network, address string) (net.Conn, error) {
						attempts++
						defer func() { connectionsFinished++ }()
						if ctx.Err() != nil {
							t.Error("started connection after cancellation")
						}
						wantDeadline := time.Now().Add(2 * time.Second)
						if end := start.Add(budget); end.Before(wantDeadline) {
							wantDeadline = end
						}
						if deadline, ok := ctx.Deadline(); !ok || !deadline.Equal(wantDeadline) {
							t.Errorf("connection deadline = %v, %v; want %v", deadline, ok, wantDeadline)
						}
						if tc.cancelRunning {
							started <- struct{}{}
						}
						<-ctx.Done()
						return nil, ctx.Err()
					},
				}
				r := NewResolver()
				transport := r.client.Transport.(*http.Transport)
				defer transport.CloseIdleConnections()
				transport.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
					if _, ok := ctx.Deadline(); ok {
						t.Error("test must exercise Transport's detached context")
					}
					return dialer.DialContext(ctx, network, address)
				}
				if _, err := r.Resolve(ctx, "http://example.com"); !errors.Is(err, errUnavailable) {
					t.Fatalf("Resolve error = %v", err)
				}
				synctest.Wait()
				if elapsed := time.Since(start); elapsed != tc.wantDuration {
					t.Errorf("elapsed = %v; want %v", elapsed, tc.wantDuration)
				}
				if lookupCalls != 1 || lookupFinished != 1 || attempts != tc.wantAttempts || connectionsFinished != attempts {
					t.Errorf("DNS started/finished = %d/%d; connections started/finished = %d/%d; want %d", lookupCalls, lookupFinished, attempts, connectionsFinished, tc.wantAttempts)
				}
				time.Sleep(10 * time.Second)
				synctest.Wait()
				if lookupCalls != 1 || attempts != tc.wantAttempts {
					t.Error("network work continued after Resolve returned")
				}
			})
		})
	}
}

func TestPublicDialCancellation(t *testing.T) {
	for _, alreadyCancelled := range []bool{true, false} {
		ctx, cancel := context.WithCancel(context.Background())
		lookupCalls, attempts := 0, 0
		dialer := publichttp.Dialer{
			Lookup: func(context.Context, string, string) ([]netip.Addr, error) {
				lookupCalls++
				return []netip.Addr{netip.MustParseAddr("1.1.1.1"), netip.MustParseAddr("8.8.8.8")}, nil
			},
			Connect: func(context.Context, string, string) (net.Conn, error) {
				attempts++
				cancel()
				return nil, errUnavailable
			},
		}
		if alreadyCancelled {
			cancel()
		}
		_, err := dialer.DialContext(ctx, "tcp", "example.com:80")
		cancel()
		want := 1
		if alreadyCancelled {
			want = 0
		}
		if err == nil || lookupCalls != want || attempts != want {
			t.Fatalf("cancelled=%v: error=%v, lookups=%d, attempts=%d", alreadyCancelled, err, lookupCalls, attempts)
		}
	}
}

func TestPublicDialPinsValidatedAddresses(t *testing.T) {
	for _, mixed := range []bool{false, true} {
		client, server := net.Pipe()
		addresses := []netip.Addr{netip.MustParseAddr("1.1.1.1"), netip.MustParseAddr("8.8.8.8")}
		if mixed {
			addresses = append(addresses, netip.MustParseAddr("127.0.0.1"))
		}
		var attempts []string
		dialer := publichttp.Dialer{
			Lookup: func(context.Context, string, string) ([]netip.Addr, error) { return addresses, nil },
			Connect: func(_ context.Context, network, address string) (net.Conn, error) {
				attempts = append(attempts, address)
				if address == "1.1.1.1:80" {
					return nil, errUnavailable
				}
				return client, nil
			},
		}
		conn, err := dialer.DialContext(context.Background(), "tcp", "example.com:80")
		client.Close()
		server.Close()
		if mixed {
			if err == nil || conn != nil || len(attempts) != 0 {
				t.Fatal("mixed public/private DNS results allowed a connection")
			}
		} else if err != nil || conn != client || strings.Join(attempts, ",") != "1.1.1.1:80,8.8.8.8:80" {
			t.Fatalf("connection=%v, error=%v, attempts=%v", conn, err, attempts)
		}
	}
}

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
		if publichttp.IsPublic(netip.MustParseAddr(ip)) {
			t.Errorf("allowed %s", ip)
		}
	}
	for _, ip := range []string{"1.1.1.1", "8.8.8.8", "2606:4700:4700::1111", "::ffff:8.8.8.8"} {
		if !publichttp.IsPublic(netip.MustParseAddr(ip)) {
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
	if _, err := publichttp.DialContext(context.Background(), "tcp", "127.0.0.1:80"); err == nil {
		t.Fatal("private connection allowed")
	}
}

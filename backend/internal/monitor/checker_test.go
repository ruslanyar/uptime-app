package monitor

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"io"
	"net"
	"net/http"
	"strings"
	"testing"
	"testing/synctest"
	"time"
	"uptime-app/backend/internal/publichttp"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

type unreadBody struct {
	t      *testing.T
	closed bool
}

func (b *unreadBody) Read([]byte) (int, error) {
	b.t.Error("checker read the final body")
	return 0, io.EOF
}
func (b *unreadBody) Close() error { b.closed = true; return nil }

func TestCheckHTTPAndHeaders(t *testing.T) {
	for _, status := range []int{200, 204, 299, 301, 403, 404, 429, 500, 503} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			body := &unreadBody{t: t}
			c := NewChecker()
			defer c.Close()
			c.client.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
				if r.Method != "GET" || r.Header.Get("User-Agent") != "UptimeApp-Monitor/1.0" || r.Header.Get("Cookie") != "" || r.Header.Get("Authorization") != "" {
					t.Fatal(r.Method, r.Header)
				}
				return &http.Response{StatusCode: status, Header: make(http.Header), Body: body, Request: r}, nil
			})
			state, result := c.Check(context.Background(), "https://example.com/path?q=1")
			want := "down"
			if status >= 200 && status < 300 {
				want = "up"
			}
			if state != want || result.HTTPStatus == nil || *result.HTTPStatus != int32(status) || result.ResponseTimeMS == nil || !body.closed || result.CheckedAt.Location() != time.UTC {
				t.Fatal(state, result, body.closed)
			}
			if state == "down" && (result.ErrorCode == nil || *result.ErrorCode != "http_status") {
				t.Fatal(result)
			}
		})
	}
}

func TestCheckNetworkErrors(t *testing.T) {
	for _, tc := range []struct {
		code string
		err  error
	}{
		{"blocked_address", publichttp.ErrBlocked}, {"dns", &net.DNSError{Err: "not found", IsNotFound: true}},
		{"connection", &net.OpError{Op: "dial", Net: "tcp", Err: errors.New("refused")}},
		{"tls", &tls.CertificateVerificationError{Err: x509.UnknownAuthorityError{}}},
		{"timeout", context.DeadlineExceeded}, {"redirect", errRedirect}, {"network", io.ErrUnexpectedEOF},
	} {
		t.Run(tc.code, func(t *testing.T) {
			c := NewChecker()
			defer c.Close()
			c.client.Transport = roundTripFunc(func(*http.Request) (*http.Response, error) { return nil, tc.err })
			state, result := c.Check(context.Background(), "https://example.com")
			want := "down"
			if tc.code == "blocked_address" {
				want = "blocked"
			}
			if state != want || result.ErrorCode == nil || *result.ErrorCode != tc.code || result.HTTPStatus != nil || result.ResponseTimeMS != nil {
				t.Fatal(state, result)
			}
		})
	}
}

func TestCheckRedirects(t *testing.T) {
	for _, tc := range []struct {
		location string
		want     string
	}{
		{"/final", "up"}, {"https://other.example/final", "up"}, {"/loop", "down"}, {"ftp://example.com/file", "down"}, {"http://user:password@example.com", "down"}, {"http://127.0.0.1/", "blocked"},
	} {
		t.Run(tc.location, func(t *testing.T) {
			c := NewChecker()
			defer c.Close()
			calls := 0
			c.client.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
				calls++
				if r.Header.Get("Referer") != "" || r.Header.Get("Cookie") != "" {
					t.Fatal(r.Header)
				}
				if r.URL.Hostname() == "127.0.0.1" {
					return nil, publichttp.ErrBlocked
				}
				status := 302
				h := make(http.Header)
				if r.URL.Path == "/final" {
					status = 200
				} else {
					h.Set("Location", tc.location)
				}
				return &http.Response{StatusCode: status, Header: h, Body: io.NopCloser(strings.NewReader("")), Request: r}, nil
			})
			state, result := c.Check(context.Background(), "http://example.com/start?secret=value")
			if state != tc.want || calls > 6 {
				t.Fatal(state, result, calls)
			}
			if state == "down" && (result.ErrorCode == nil || *result.ErrorCode != "redirect") {
				t.Fatal(result)
			}
		})
	}
}

func TestCheckBudgetAndCancellation(t *testing.T) {
	for _, cancelCaller := range []bool{false, true} {
		synctest.Test(t, func(t *testing.T) {
			c := NewChecker()
			defer c.Close()
			parent, cancel := context.WithCancel(context.Background())
			defer cancel()
			if cancelCaller {
				go func() { time.Sleep(time.Second); cancel() }()
			}
			c.client.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) { <-r.Context().Done(); return nil, r.Context().Err() })
			start := time.Now()
			_, result := c.Check(parent, "https://example.com")
			duration := 10 * time.Second
			code := "timeout"
			if cancelCaller {
				duration = time.Second
				code = "network"
			}
			if time.Since(start) != duration || result.ErrorCode == nil || *result.ErrorCode != code {
				t.Fatal(time.Since(start), result)
			}
		})
	}
}

func TestCheckLiteralPrivateAddressWithoutRequest(t *testing.T) {
	c := NewChecker()
	defer c.Close()
	c.client.Transport = roundTripFunc(func(*http.Request) (*http.Response, error) { t.Fatal("requested private IP"); return nil, nil })
	for _, url := range []string{"http://127.0.0.1", "http://10.0.0.1", "http://[::1]", "http://169.254.169.254"} {
		state, result := c.Check(context.Background(), url)
		if state != "blocked" || *result.ErrorCode != "blocked_address" {
			t.Fatal(state, result)
		}
	}
}

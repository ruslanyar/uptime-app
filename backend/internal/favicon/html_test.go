package favicon

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestBaseHrefDiscovery(t *testing.T) {
	for _, tc := range []struct {
		name, bases, iconPath string
	}{
		{"missing href", `<base target="_blank"><base href="/assets/">`, "/assets/logo.svg"},
		{"only missing href", `<base target="_blank">`, "/nested/logo.svg"},
		{"empty href", `<base href=""><base href="/wrong/">`, "/nested/logo.svg"},
		{"invalid href", `<base href="file:///blocked"><base href="/assets/">`, "/assets/logo.svg"},
		{"first valid href", `<base href="/assets/"><base href="/wrong/">`, "/assets/logo.svg"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, redirect := range []bool{false, true} {
				t.Run(map[bool]string{false: "direct", true: "redirect"}[redirect], func(t *testing.T) {
					var requests []string
					server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						requests = append(requests, r.URL.Path)
						switch r.URL.Path {
						case "/old/page":
							http.Redirect(w, r, "/nested/page", http.StatusFound)
						case "/nested/page":
							_, _ = io.WriteString(w, tc.bases+`<link rel="icon" href="logo.svg">`)
						case tc.iconPath:
							_, _ = io.WriteString(w, svg)
						default:
							w.WriteHeader(http.StatusNotFound)
						}
					}))
					defer server.Close()
					path := "/nested/page"
					want := path + "," + tc.iconPath
					if redirect {
						path = "/old/page"
						want = path + "," + want
					}
					icon, err := (&Resolver{client: server.Client()}).Resolve(context.Background(), server.URL+path)
					if err != nil || icon.Type != "image/svg+xml" || string(icon.Data) != svg {
						t.Fatalf("icon=%v, error=%v, requests=%v", icon, err, requests)
					}
					if got := strings.Join(requests, ","); got != want {
						t.Fatalf("requests = %s; want %s", got, want)
					}
				})
			}
		})
	}
}

func TestLargeHTMLDiscovery(t *testing.T) {
	for _, redirect := range []bool{false, true} {
		t.Run(map[bool]string{false: "direct", true: "redirect"}[redirect], func(t *testing.T) {
			var requests []string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests = append(requests, r.URL.RequestURI())
				switch r.URL.Path {
				case "/new/nested/page":
					_, _ = io.WriteString(w, `<html><head><base href="file:///blocked"><base href="../assets/"><base href="/wrong/"><link rel="icon" href="logo.svg?a=1&amp;b=2"></head><body>`+strings.Repeat("x", 2*maxBody)+`</body></html>`)
				case "/new/assets/logo.svg":
					_, _ = io.WriteString(w, svg)
				default:
					w.WriteHeader(http.StatusNotFound)
				}
			}))
			defer server.Close()
			raw := server.URL + "/new/nested/page"
			if redirect {
				original := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if r.URL.Path != "/old/page" {
						t.Errorf("icon requested from original origin: %s", r.URL.Path)
						w.WriteHeader(http.StatusNotFound)
						return
					}
					http.Redirect(w, r, server.URL+"/new/nested/page", http.StatusFound)
				}))
				defer original.Close()
				raw = original.URL + "/old/page"
			}
			icon, err := (&Resolver{client: server.Client()}).Resolve(context.Background(), raw)
			if err != nil || icon.Type != "image/svg+xml" || string(icon.Data) != svg {
				t.Fatalf("declared icon not discovered: icon=%v, error=%v", icon, err)
			}
			if got := strings.Join(requests, ","); got != "/new/nested/page,/new/assets/logo.svg?a=1&b=2" {
				t.Fatalf("requests = %s", got)
			}
		})
	}
}

func TestLargeHTMLFallback(t *testing.T) {
	for _, tc := range []struct {
		name, head string
		available  bool
	}{
		{"no declared icon", "", true},
		{"no available icon", "", false},
		{"candidate limit", strings.Repeat(`<link rel="icon" href="/missing">`, 6) + `<link rel="icon" href="/seventh">`, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var requests []string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests = append(requests, r.URL.Path)
				if r.URL.Path == "/page" {
					_, _ = io.WriteString(w, `<html><head>`+tc.head+`</head><body>`+strings.Repeat("x", 2*maxBody)+`<link rel="icon" href="/outside-prefix"></body></html>`)
					return
				}
				if r.URL.Path == "/favicon.ico" && tc.available {
					_, _ = io.WriteString(w, svg)
					return
				}
				if r.URL.Path == "/outside-prefix" || r.URL.Path == "/seventh" {
					t.Errorf("requested icon outside discovery limits: %s", r.URL.Path)
					_, _ = io.WriteString(w, svg)
					return
				}
				w.WriteHeader(http.StatusNotFound)
			}))
			defer server.Close()
			icon, err := (&Resolver{client: server.Client()}).Resolve(context.Background(), server.URL+"/page")
			if tc.available {
				if err != nil || string(icon.Data) != svg {
					t.Fatal(icon, err)
				}
			} else if err != errUnavailable {
				t.Fatalf("missing icon error = %v", err)
			}
			want := "/page,/favicon.ico"
			if tc.head != "" {
				want = "/page,/missing,/favicon.ico"
			}
			if got := strings.Join(requests, ","); got != want {
				t.Fatalf("requests = %s; want %s", got, want)
			}
		})
	}
}

func TestOversizedImageFallback(t *testing.T) {
	for _, fallbackAvailable := range []bool{false, true} {
		var requests []string
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requests = append(requests, r.URL.Path)
			switch r.URL.Path {
			case "/page":
				_, _ = io.WriteString(w, `<link rel="icon" href="/oversized">`)
			case "/oversized":
				// The first 512 KiB form a valid SVG. Accepting only that prefix
				// would hide the fact that the complete image exceeds the limit.
				_, _ = io.WriteString(w, svg+strings.Repeat(" ", maxBody-len(svg))+" ")
			case "/favicon.ico":
				if fallbackAvailable {
					_, _ = io.WriteString(w, svg)
				} else {
					w.WriteHeader(http.StatusNotFound)
				}
			}
		}))
		icon, err := (&Resolver{client: server.Client()}).Resolve(context.Background(), server.URL+"/page")
		server.Close()
		if fallbackAvailable {
			if err != nil || string(icon.Data) != svg {
				t.Fatal(icon, err)
			}
		} else if err != errUnavailable || len(icon.Data) != 0 {
			t.Fatal("oversized icon was accepted", icon, err)
		}
		if got := strings.Join(requests, ","); got != "/page,/oversized,/favicon.ico" {
			t.Fatalf("requests = %s", got)
		}
	}
}

type countedBody struct {
	io.Reader
	read   int
	closed bool
}

func (b *countedBody) Read(p []byte) (int, error) {
	n, err := b.Reader.Read(p)
	b.read += n
	return n, err
}

func (b *countedBody) Close() error {
	b.closed = true
	return nil
}

type responseTransport struct{ body io.ReadCloser }

func (tr responseTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return &http.Response{StatusCode: http.StatusOK, Body: tr.body, Request: req}, nil
}

func TestResponseReadLimits(t *testing.T) {
	for _, limit := range []int64{maxBody, maxBody + 1} {
		body := &countedBody{Reader: strings.NewReader(strings.Repeat("x", 3*maxBody))}
		r := &Resolver{client: &http.Client{Transport: responseTransport{body}}}
		u, _ := url.Parse("https://example.com")
		prefix, final, err := r.fetch(context.Background(), u, limit)
		if err != nil || final.String() != u.String() || int64(len(prefix)) != limit || int64(body.read) != limit || !body.closed {
			t.Fatalf("limit=%d: bytes=%d, read=%d, closed=%v, final=%v, error=%v", limit, len(prefix), body.read, body.closed, final, err)
		}
	}
}

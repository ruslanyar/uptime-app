// Package favicon discovers site icons using bounded requests to public addresses.
package favicon

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"golang.org/x/net/html"
	"uptime-app/backend/internal/publichttp"
)

const maxBody = 512 * 1024

var errUnavailable = errors.New("favicon unavailable")

type Icon struct {
	Data []byte
	Type string
}

type Resolver struct{ client *http.Client }

// NewResolver blocks private destinations at dial time, also after redirects.
// DNS results are pinned to the connection to prevent DNS rebinding.
func NewResolver() *Resolver {
	transport := publichttp.NewTransport()
	transport.ResponseHeaderTimeout = 2 * time.Second
	return &Resolver{client: &http.Client{
		Transport: transport,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 3 || !validURL(req.URL) {
				return errUnavailable
			}
			return nil
		},
	}}
}

func validURL(u *url.URL) bool {
	return (u.Scheme == "http" || u.Scheme == "https") && u.Hostname() != "" && u.User == nil
}

// fetch returns a bounded prefix and the final URL, even on a response read error.
// HTML discovery can use a prefix; image callers must probe for oversize files.
func (r *Resolver) fetch(ctx context.Context, u *url.URL, readLimit int64) ([]byte, *url.URL, error) {
	if !validURL(u) {
		return nil, nil, errUnavailable
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, nil, errUnavailable
	}
	req.Header.Set("User-Agent", "UptimeApp-Favicon/1.0")
	resp, err := r.client.Do(req)
	if err != nil {
		return nil, nil, errUnavailable
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, resp.Request.URL, errUnavailable
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, readLimit))
	if err != nil || len(body) == 0 {
		return nil, resp.Request.URL, errUnavailable
	}
	return body, resp.Request.URL, nil
}

func candidates(body []byte, page *url.URL) []*url.URL {
	root, err := html.Parse(bytes.NewReader(body))
	if err != nil {
		return nil
	}
	base := page
	var links []string
	var visit func(*html.Node)
	visit = func(n *html.Node) {
		if n.Type == html.ElementNode {
			attrs := map[string]string{}
			for _, a := range n.Attr {
				attrs[a.Key] = a.Val
			}
			if n.Data == "base" && base == page {
				// A target-only base does not set the document URL; an explicit
				// empty href does and must prevent later bases from overriding it.
				if href, present := attrs["href"]; present {
					if u, err := page.Parse(href); err == nil && validURL(u) {
						base = u
					}
				}
			}
			if n.Data == "link" && len(links) < 6 {
				for _, rel := range strings.Fields(strings.ToLower(attrs["rel"])) {
					if rel == "icon" || rel == "apple-touch-icon" {
						if attrs["href"] != "" {
							links = append(links, attrs["href"])
						}
						break
					}
				}
			}
		}
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			visit(child)
		}
	}
	visit(root)
	var result []*url.URL
	for _, link := range links {
		if u, err := base.Parse(link); err == nil && validURL(u) {
			result = append(result, u)
		}
	}
	return result
}

// Resolve tries declared icons then /favicon.ico within a shared five-second budget.
// Caller cancellation and the budget interrupt DNS, connections and response reads.
// Missing, blocked, cancelled and failed icons return an error.
func (r *Resolver) Resolve(ctx context.Context, raw string) (Icon, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	ctx = publichttp.WithBudget(ctx)
	page, err := url.Parse(raw)
	if err != nil || !validURL(page) {
		return Icon{}, errUnavailable
	}
	var urls []*url.URL
	body, final, err := r.fetch(ctx, page, maxBody)
	// The final origin owns the fallback even when its HTML cannot be read.
	if final != nil {
		page = final
	}
	if err == nil {
		urls = candidates(body, page)
	}
	fallback := &url.URL{Scheme: page.Scheme, Host: page.Host, Path: "/favicon.ico"}
	urls = append(urls, fallback)
	seen := map[string]bool{}
	for _, u := range urls {
		if seen[u.String()] || ctx.Err() != nil {
			continue
		}
		seen[u.String()] = true
		// The extra byte detects oversized images without accepting a truncated
		// prefix, even if that prefix happens to decode as a complete image.
		body, _, err := r.fetch(ctx, u, maxBody+1)
		if err == nil && len(body) <= maxBody {
			if kind := imageType(ctx, body); kind != "" {
				return Icon{Data: body, Type: kind}, nil
			}
		}
	}
	return Icon{}, errUnavailable
}

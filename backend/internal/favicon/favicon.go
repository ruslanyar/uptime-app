// Package favicon discovers site icons using bounded requests to public addresses.
package favicon

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
	"time"

	"golang.org/x/net/html"
)

const maxBody = 512 * 1024

var errUnavailable = errors.New("favicon unavailable")

type Icon struct {
	Data []byte
	Type string
}

type Resolver struct{ client *http.Client }

type resolveContextKey struct{}

type publicDialer struct {
	lookup  func(context.Context, string, string) ([]netip.Addr, error)
	connect func(context.Context, string, string) (net.Conn, error)
}

// NewResolver blocks private destinations at dial time, also after redirects.
// DNS results are pinned to the connection to prevent DNS rebinding.
func NewResolver() *Resolver {
	transport := &http.Transport{
		DialContext:           publicDial,
		TLSHandshakeTimeout:   2 * time.Second,
		ResponseHeaderTimeout: 2 * time.Second,
		MaxIdleConns:          32,
		IdleConnTimeout:       30 * time.Second,
	}
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

var blocked = []netip.Prefix{
	netip.MustParsePrefix("0.0.0.0/8"),
	netip.MustParsePrefix("100.64.0.0/10"),
	netip.MustParsePrefix("192.0.0.0/24"),
	netip.MustParsePrefix("192.0.2.0/24"),
	netip.MustParsePrefix("198.18.0.0/15"),
	netip.MustParsePrefix("198.51.100.0/24"),
	netip.MustParsePrefix("203.0.113.0/24"),
	netip.MustParsePrefix("240.0.0.0/4"),
}

func publicIP(addr netip.Addr) bool {
	addr = addr.Unmap()
	if !addr.IsGlobalUnicast() || addr.IsPrivate() || addr.IsLoopback() || addr.IsLinkLocalUnicast() {
		return false
	}
	if addr.Is6() {
		// Exclude IPv4 translation/tunnelling and special-purpose IPv6 ranges.
		return netip.MustParsePrefix("2000::/3").Contains(addr) &&
			!netip.MustParsePrefix("2001::/23").Contains(addr) &&
			!netip.MustParsePrefix("2001:db8::/32").Contains(addr) &&
			!netip.MustParsePrefix("2002::/16").Contains(addr) &&
			!netip.MustParsePrefix("3fff::/20").Contains(addr)
	}
	for _, prefix := range blocked {
		if prefix.Contains(addr) {
			return false
		}
	}
	return true
}

func publicDial(ctx context.Context, network, address string) (net.Conn, error) {
	dialer := publicDialer{
		lookup:  net.DefaultResolver.LookupNetIP,
		connect: (&net.Dialer{}).DialContext,
	}
	return dialer.dial(ctx, network, address)
}

func (d publicDialer) dial(ctx context.Context, network, address string) (net.Conn, error) {
	// Transport detaches request cancellation and deadlines but retains values.
	// Restore the Resolve budget while also honoring Transport's own cancellation.
	if resolveCtx, ok := ctx.Value(resolveContextKey{}).(context.Context); ok {
		operationCtx, cancel := context.WithCancel(resolveCtx)
		stop := context.AfterFunc(ctx, cancel)
		defer stop()
		defer cancel()
		ctx = operationCtx
	}
	if ctx.Err() != nil {
		return nil, errUnavailable
	}
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, errUnavailable
	}
	addresses, err := d.lookup(ctx, "ip", host)
	if err != nil || len(addresses) == 0 {
		return nil, errUnavailable
	}
	for _, addr := range addresses {
		if !publicIP(addr) {
			return nil, errUnavailable
		}
	}
	for _, addr := range addresses {
		if ctx.Err() != nil {
			return nil, errUnavailable
		}
		attemptCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
		conn, err := d.connect(attemptCtx, network, net.JoinHostPort(addr.String(), port))
		cancel()
		if err == nil {
			return conn, nil
		}
	}
	return nil, errUnavailable
}

func validURL(u *url.URL) bool {
	return (u.Scheme == "http" || u.Scheme == "https") && u.Hostname() != "" && u.User == nil
}

func (r *Resolver) fetch(ctx context.Context, u *url.URL) ([]byte, *url.URL, error) {
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
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBody+1))
	if err != nil || len(body) == 0 || len(body) > maxBody {
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
				if u, err := page.Parse(attrs["href"]); err == nil && validURL(u) {
					base = u
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
	ctx = context.WithValue(ctx, resolveContextKey{}, ctx)
	page, err := url.Parse(raw)
	if err != nil || !validURL(page) {
		return Icon{}, errUnavailable
	}
	var urls []*url.URL
	body, final, err := r.fetch(ctx, page)
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
		body, _, err := r.fetch(ctx, u)
		if err == nil {
			if kind := imageType(ctx, body); kind != "" {
				return Icon{Data: body, Type: kind}, nil
			}
		}
	}
	return Icon{}, errUnavailable
}

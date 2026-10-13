// Package publichttp restricts outbound HTTP connections to validated public IPs.
package publichttp

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/netip"
	"time"
)

var ErrBlocked = errors.New("destination is not public")

type budgetKey struct{}

// WithBudget restores request cancellation in Transport's detached dial context.
func WithBudget(ctx context.Context) context.Context { return context.WithValue(ctx, budgetKey{}, ctx) }

// Dialer validates every DNS answer before connecting to a pinned IP.
// Lookup and Connect must honor their context; connection attempts share its budget.
type Dialer struct {
	Lookup  func(context.Context, string, string) ([]netip.Addr, error)
	Connect func(context.Context, string, string) (net.Conn, error)
}

// NewTransport uses no environment proxy and bounds each connection attempt.
// Callers must wrap request contexts with WithBudget and close idle connections.
func NewTransport() *http.Transport {
	return &http.Transport{DialContext: DialContext, TLSHandshakeTimeout: 2 * time.Second, MaxIdleConns: 32, IdleConnTimeout: 30 * time.Second, MaxResponseHeaderBytes: 64 * 1024}
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

func IsPublic(addr netip.Addr) bool {
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

func DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	dialer := Dialer{
		Lookup:  net.DefaultResolver.LookupNetIP,
		Connect: (&net.Dialer{}).DialContext,
	}
	return dialer.DialContext(ctx, network, address)
}

func (d Dialer) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	// Transport detaches request cancellation and deadlines but retains values.
	// Restore the request budget while also honoring Transport's own cancellation.
	if resolveCtx, ok := ctx.Value(budgetKey{}).(context.Context); ok {
		operationCtx, cancel := context.WithCancel(resolveCtx)
		stop := context.AfterFunc(ctx, cancel)
		defer stop()
		defer cancel()
		ctx = operationCtx
	}
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, err
	}
	addresses, err := d.Lookup(ctx, "ip", host)
	if err != nil {
		return nil, err
	}
	if len(addresses) == 0 {
		return nil, &net.DNSError{Err: "no addresses", Name: host, IsNotFound: true}
	}
	for _, addr := range addresses {
		if !IsPublic(addr) {
			return nil, ErrBlocked
		}
	}
	var lastErr error
	for _, addr := range addresses {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		attemptCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
		conn, err := d.Connect(attemptCtx, network, net.JoinHostPort(addr.String(), port))
		cancel()
		if err == nil {
			return conn, nil
		}
		lastErr = err
	}
	return nil, lastErr
}

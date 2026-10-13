package monitor

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"time"
	"uptime-app/backend/internal/publichttp"
)

type Monitoring struct {
	Status      string       `json:"status" binding:"required" enums:"unknown,up,down,blocked"`
	LastCheck   *CheckResult `json:"last_check" binding:"required" extensions:"x-nullable"`
	NextCheckAt time.Time    `json:"next_check_at" binding:"required" format:"date-time"`
	IsStale     bool         `json:"is_stale" binding:"required"`
}
type CheckResult struct {
	CheckedAt      time.Time `json:"checked_at" binding:"required" format:"date-time"`
	HTTPStatus     *int32    `json:"http_status" binding:"required" extensions:"x-nullable"`
	ResponseTimeMS *int64    `json:"response_time_ms" binding:"required" extensions:"x-nullable"`
	ErrorCode      *string   `json:"error_code" binding:"required" extensions:"x-nullable" enums:"http_status,dns,connection,tls,timeout,redirect,blocked_address,network"`
}

// Checker measures HTTP response headers, not the full page or its functionality.
// Use NewChecker for production; client injection is reserved for controlled tests.
type Checker struct{ client *http.Client }

var errRedirect = errors.New("invalid or excessive redirect")

func NewChecker() *Checker {
	return &Checker{client: &http.Client{
		Transport: publichttp.NewTransport(),
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) > 5 || req.URL.User != nil || (req.URL.Scheme != "http" && req.URL.Scheme != "https") || req.URL.Hostname() == "" {
				return errRedirect
			}
			// Checks never carry state or the previous URL's query in a Referer header.
			req.Header.Del("Referer")
			return nil
		},
	}}
}

// Close releases idle connections after worker shutdown.
func (c *Checker) Close() { c.client.CloseIdleConnections() }

// Check honors caller cancellation; cancelled work must not be persisted.
// All network stages and redirects share a ten-second budget. Bodies are closed unread.
func (c *Checker) Check(parent context.Context, raw string) (string, CheckResult) {
	ctx, cancel := context.WithTimeout(parent, 10*time.Second)
	defer cancel()
	ctx = publichttp.WithBudget(ctx)
	start := time.Now()
	result := CheckResult{}
	finish := func(code string) (string, CheckResult) {
		result.CheckedAt = time.Now().UTC()
		result.ErrorCode = &code
		if code == "blocked_address" {
			return "blocked", result
		}
		return "down", result
	}
	u, err := url.Parse(raw)
	if err != nil || u.User != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" {
		return finish("network")
	}
	if ip, e := netip.ParseAddr(u.Hostname()); e == nil && !publichttp.IsPublic(ip) {
		return finish("blocked_address")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, raw, nil)
	if err != nil {
		return finish("network")
	}
	req.Header.Set("User-Agent", "UptimeApp-Monitor/1.0")
	resp, err := c.client.Do(req)
	elapsed := time.Since(start).Milliseconds()
	if err != nil {
		if resp != nil {
			resp.Body.Close()
		}
		return finish(checkError(err, ctx))
	}
	resp.Body.Close()
	result.CheckedAt = time.Now().UTC()
	status := int32(resp.StatusCode)
	result.HTTPStatus = &status
	result.ResponseTimeMS = &elapsed
	if status >= 200 && status < 300 {
		return "up", result
	}
	return finish("http_status")
}

func checkError(err error, ctx context.Context) string {
	if errors.Is(err, publichttp.ErrBlocked) {
		return "blocked_address"
	}
	if errors.Is(err, errRedirect) {
		return "redirect"
	}
	var tlsVerification *tls.CertificateVerificationError
	var unknownAuthority x509.UnknownAuthorityError
	var hostname x509.HostnameError
	var invalidCertificate x509.CertificateInvalidError
	var record tls.RecordHeaderError
	if errors.As(err, &tlsVerification) || errors.As(err, &unknownAuthority) || errors.As(err, &hostname) || errors.As(err, &invalidCertificate) || errors.As(err, &record) {
		return "tls"
	}
	var timeout net.Error
	if errors.Is(ctx.Err(), context.DeadlineExceeded) || errors.Is(err, context.DeadlineExceeded) || (errors.As(err, &timeout) && timeout.Timeout()) {
		return "timeout"
	}
	var dns *net.DNSError
	if errors.As(err, &dns) {
		return "dns"
	}
	var connection *net.OpError
	if errors.As(err, &connection) {
		return "connection"
	}
	return "network"
}

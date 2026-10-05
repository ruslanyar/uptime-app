package config

import (
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	DatabaseURL, HTTPAddr, JWTSecret, JWTIssuer, JWTAudience string
	AllowedOrigins                                           map[string]bool
	CookieSecure                                             bool
	CookieSameSite                                           http.SameSite
}

func ValidOrigin(s string) bool {
	u, e := url.Parse(s)
	return e == nil && (u.Scheme == "http" || u.Scheme == "https") && u.Host != "" && u.Hostname() != "" && u.User == nil && u.Path == "" && u.RawQuery == "" && !u.ForceQuery && u.Fragment == "" && !strings.ContainsAny(s, "*#?\\") && s == u.Scheme+"://"+u.Host
}

func Load() (Config, error) { return Read(os.Getenv) }
func Read(get func(string) string) (Config, error) {
	c := Config{DatabaseURL: get("DATABASE_URL"), HTTPAddr: get("HTTP_ADDR"), JWTSecret: get("JWT_SECRET"), JWTIssuer: get("JWT_ISSUER"), JWTAudience: get("JWT_AUDIENCE"), AllowedOrigins: map[string]bool{}}
	for _, v := range []struct{ k, v string }{{"DATABASE_URL", c.DatabaseURL}, {"JWT_SECRET", c.JWTSecret}, {"JWT_ISSUER", c.JWTIssuer}, {"JWT_AUDIENCE", c.JWTAudience}, {"ALLOWED_ORIGINS", get("ALLOWED_ORIGINS")}} {
		if strings.TrimSpace(v.v) == "" {
			return c, fmt.Errorf("%s is required", v.k)
		}
	}
	if len(c.JWTSecret) < 32 {
		return c, fmt.Errorf("JWT_SECRET must contain at least 32 random bytes")
	}
	u, e := url.Parse(c.DatabaseURL)
	if e != nil || (u.Scheme != "postgres" && u.Scheme != "postgresql") || u.Host == "" {
		return c, fmt.Errorf("DATABASE_URL must be a PostgreSQL URL")
	}
	if c.HTTPAddr == "" {
		c.HTTPAddr = ":8080"
	}
	if _, _, e = net.SplitHostPort(c.HTTPAddr); e != nil {
		return c, fmt.Errorf("invalid HTTP_ADDR")
	}
	for _, o := range strings.Split(get("ALLOWED_ORIGINS"), ",") {
		o = strings.TrimSpace(o)
		if !ValidOrigin(o) {
			return c, fmt.Errorf("invalid ALLOWED_ORIGINS entry: %q", o)
		}
		c.AllowedOrigins[o] = true
	}
	c.CookieSecure = true
	if s := get("COOKIE_SECURE"); s != "" {
		c.CookieSecure, e = strconv.ParseBool(s)
		if e != nil {
			return c, fmt.Errorf("invalid COOKIE_SECURE")
		}
	}
	switch get("COOKIE_SAME_SITE") {
	case "", "none":
		c.CookieSameSite = http.SameSiteNoneMode
	case "lax":
		c.CookieSameSite = http.SameSiteLaxMode
	case "strict":
		c.CookieSameSite = http.SameSiteStrictMode
	default:
		return c, fmt.Errorf("invalid COOKIE_SAME_SITE")
	}
	if c.CookieSameSite == http.SameSiteNoneMode && !c.CookieSecure {
		return c, fmt.Errorf("COOKIE_SAME_SITE=none requires COOKIE_SECURE=true")
	}
	return c, nil
}

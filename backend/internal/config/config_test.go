package config

import (
	"net/http"
	"strings"
	"testing"
)

func env() map[string]string {
	return map[string]string{"DATABASE_URL": "postgres://user:pass@localhost/db", "JWT_SECRET": strings.Repeat("x", 32), "JWT_ISSUER": "uptime", "JWT_AUDIENCE": "client", "ALLOWED_ORIGINS": "http://localhost:3000,https://client.example"}
}
func TestConfig(t *testing.T) {
	c, e := Read(func(k string) string { return env()[k] })
	if e != nil || !c.CookieSecure || c.CookieSameSite != http.SameSiteNoneMode {
		t.Fatalf("default: %+v %v", c, e)
	}
	for _, key := range []string{"DATABASE_URL", "JWT_SECRET", "JWT_ISSUER", "JWT_AUDIENCE", "ALLOWED_ORIGINS"} {
		t.Run(key, func(t *testing.T) {
			v := env()
			delete(v, key)
			if _, e := Read(func(k string) string { return v[k] }); e == nil {
				t.Fatal("missing accepted")
			}
		})
	}
	for _, tc := range []struct{ k, v string }{{"JWT_SECRET", "short"}, {"DATABASE_URL", "http://db"}, {"HTTP_ADDR", "bad"}, {"COOKIE_SECURE", "maybe"}, {"COOKIE_SECURE", "false"}, {"COOKIE_SAME_SITE", "invalid"}, {"ALLOWED_ORIGINS", "https://*.example"}, {"ALLOWED_ORIGINS", "null"}, {"ALLOWED_ORIGINS", "https://example/"}, {"ALLOWED_ORIGINS", "https://example?"}, {"ALLOWED_ORIGINS", "https://example#"}, {"ALLOWED_ORIGINS", "https://user@example"}} {
		t.Run(tc.k+tc.v, func(t *testing.T) {
			v := env()
			v[tc.k] = tc.v
			if _, e := Read(func(k string) string { return v[k] }); e == nil {
				t.Fatal("invalid accepted")
			}
		})
	}
	v := env()
	v["COOKIE_SECURE"] = "false"
	v["COOKIE_SAME_SITE"] = "lax"
	c, e = Read(func(k string) string { return v[k] })
	if e != nil || c.CookieSecure || c.CookieSameSite != http.SameSiteLaxMode {
		t.Fatal(c, e)
	}
	v["COOKIE_SAME_SITE"] = "strict"
	if _, e = Read(func(k string) string { return v[k] }); e != nil {
		t.Fatal(e)
	}
}

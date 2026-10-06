package auth

import (
	"github.com/golang-jwt/jwt/v5"
	"strings"
	"testing"
	"time"
)

func TestValidation(t *testing.T) {
	name, email, e := Validate("  Имя  ", "  USER@Example.com  ", "  long password  ")
	if e != nil || name != "Имя" || email != "user@example.com" {
		t.Fatal(name, email, e)
	}
	for _, tc := range []struct {
		name, email, password string
		ok                    bool
	}{{"nn", "a@b.com", strings.Repeat("x", 14), false}, {"nn", "a@b.com", strings.Repeat("x", 15), true}, {"nn", "a@b.com", strings.Repeat("я", 128), true}, {"nn", "a@b.com", strings.Repeat("x", 129), false}, {" ", "a@b.com", strings.Repeat("x", 15), false}, {strings.Repeat("я", 50), "a@b.com", strings.Repeat("x", 15), true}, {strings.Repeat("я", 51), "a@b.com", strings.Repeat("x", 15), false}, {"nn", "Person <a@b.com>", strings.Repeat("x", 15), false}, {"nn", "bad", strings.Repeat("x", 15), false}, {"nn", strings.Repeat("a", 250) + "@b.com", strings.Repeat("x", 15), false}} {
		_, _, e := Validate(tc.name, tc.email, tc.password)
		if (e == nil) != tc.ok {
			t.Errorf("validation %+v: %v", tc, e)
		}
	}
}
func TestPassword(t *testing.T) {
	p := "  password with spaces  "
	h, e := HashPassword(p)
	if e != nil {
		t.Fatal(e)
	}
	h2, _ := HashPassword(p)
	if h == h2 || !CheckPassword(h, p) || CheckPassword(h, strings.TrimSpace(p)) || CheckPassword(h, "wrong") {
		t.Fatal("password verification")
	}
	for _, bad := range []string{"", "bad", strings.Replace(h, "m=19456", "m=999999999", 1), strings.Replace(h, "v=19", "v=16", 1)} {
		if CheckPassword(bad, p) {
			t.Fatal("malformed accepted")
		}
	}
}
func TestTokens(t *testing.T) {
	tokens := NewTokens(strings.Repeat("s", 32), "issuer", "audience")
	id := NewID()
	raw, e := tokens.Issue(id)
	if e != nil {
		t.Fatal(e)
	}
	got, e := tokens.Verify(raw)
	if e != nil || got != id {
		t.Fatal(got, e)
	}
	claims := func() jwt.RegisteredClaims {
		now := time.Now()
		return jwt.RegisteredClaims{Subject: id, Issuer: "issuer", Audience: jwt.ClaimStrings{"audience"}, IssuedAt: jwt.NewNumericDate(now), ExpiresAt: jwt.NewNumericDate(now.Add(time.Hour)), ID: NewID()}
	}
	for _, tc := range []struct {
		name   string
		change func(*jwt.RegisteredClaims)
		method jwt.SigningMethod
		key    string
	}{{"signature", nil, jwt.SigningMethodHS256, strings.Repeat("z", 32)}, {"algorithm", nil, jwt.SigningMethodHS384, strings.Repeat("s", 32)}, {"issuer", func(c *jwt.RegisteredClaims) { c.Issuer = "bad" }, nil, ""}, {"audience", func(c *jwt.RegisteredClaims) { c.Audience = jwt.ClaimStrings{"bad"} }, nil, ""}, {"expired", func(c *jwt.RegisteredClaims) { c.ExpiresAt = jwt.NewNumericDate(time.Now().Add(-time.Hour)) }, nil, ""}, {"no expiry", func(c *jwt.RegisteredClaims) { c.ExpiresAt = nil }, nil, ""}, {"no iat", func(c *jwt.RegisteredClaims) { c.IssuedAt = nil }, nil, ""}, {"future iat", func(c *jwt.RegisteredClaims) { c.IssuedAt = jwt.NewNumericDate(time.Now().Add(time.Hour)) }, nil, ""}, {"subject", func(c *jwt.RegisteredClaims) { c.Subject = "bad" }, nil, ""}, {"jti", func(c *jwt.RegisteredClaims) { c.ID = "" }, nil, ""}} {
		t.Run(tc.name, func(t *testing.T) {
			c := claims()
			if tc.change != nil {
				tc.change(&c)
			}
			if tc.method == nil {
				tc.method = jwt.SigningMethodHS256
			}
			if tc.key == "" {
				tc.key = strings.Repeat("s", 32)
			}
			raw, e := jwt.NewWithClaims(tc.method, c).SignedString([]byte(tc.key))
			if e != nil {
				t.Fatal(e)
			}
			if _, e = tokens.Verify(raw); e == nil {
				t.Fatal("accepted invalid token")
			}
		})
	}
}

package auth

import (
	"github.com/golang-jwt/jwt/v5"
	"time"
)

type Tokens struct {
	secret           []byte
	issuer, audience string
}

func NewTokens(secret, issuer, audience string) *Tokens {
	return &Tokens{[]byte(secret), issuer, audience}
}
func (t *Tokens) Issue(id string) (string, error) {
	now := time.Now()
	return jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.RegisteredClaims{Subject: id, Issuer: t.issuer, Audience: jwt.ClaimStrings{t.audience}, IssuedAt: jwt.NewNumericDate(now), ExpiresAt: jwt.NewNumericDate(now.Add(AccessTTL)), ID: NewID()}).SignedString(t.secret)
}
func (t *Tokens) Verify(raw string) (string, error) {
	c := new(jwt.RegisteredClaims)
	token, e := jwt.ParseWithClaims(raw, c, func(token *jwt.Token) (any, error) { return t.secret, nil }, jwt.WithValidMethods([]string{"HS256"}), jwt.WithIssuer(t.issuer), jwt.WithAudience(t.audience), jwt.WithExpirationRequired(), jwt.WithIssuedAt())
	if e != nil || !token.Valid || !ValidID(c.Subject) || c.IssuedAt == nil || !ValidID(c.ID) {
		return "", ErrUnauthorized
	}
	return c.Subject, nil
}

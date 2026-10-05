package authcontroller

import (
	"net/http"
	"time"
)

func (a *Controller) cookie(w http.ResponseWriter, raw string, expires time.Time) {
	maxAge := int(time.Until(expires).Seconds())
	if raw == "" {
		maxAge = -1
		expires = time.Unix(1, 0)
	} else if maxAge < 1 {
		maxAge = 1
	}
	http.SetCookie(w, &http.Cookie{Name: "refresh_token", Value: raw, Path: "/api/v1/auth", HttpOnly: true, Secure: a.cfg.CookieSecure, SameSite: a.cfg.CookieSameSite, Expires: expires, MaxAge: maxAge})
}
func refreshCookie(r *http.Request) string {
	c, e := r.Cookie("refresh_token")
	if e != nil {
		return ""
	}
	return c.Value
}

package authcontroller

import (
	"net/http"
	"time"
)

func (a *Controller) cookie(w http.ResponseWriter, name, path, raw string, expires time.Time) {
	maxAge := int(time.Until(expires).Seconds())
	if raw == "" {
		maxAge = -1
		expires = time.Unix(1, 0)
	} else if maxAge < 1 {
		// MaxAge=0 would turn an expiring token into a browser-session cookie.
		maxAge = 1
	}
	http.SetCookie(w, &http.Cookie{Name: name, Value: raw, Path: path, HttpOnly: true, Secure: a.cfg.CookieSecure, SameSite: a.cfg.CookieSameSite, Expires: expires, MaxAge: maxAge})
}

func (a *Controller) clearCookies(w http.ResponseWriter) {
	// Deletion must use the issuance paths to target the existing cookies.
	a.cookie(w, "access_token", "/api/v1", "", time.Time{})
	a.cookie(w, "refresh_token", "/api/v1/auth", "", time.Time{})
}
func refreshCookie(r *http.Request) string {
	c, e := r.Cookie("refresh_token")
	if e != nil {
		return ""
	}
	return c.Value
}

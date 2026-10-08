package request

import (
	"net/http"
	"strings"
	"uptime-app/backend/internal/auth"
)

// AccessToken gives Authorization precedence; malformed headers never fall back
// to cookies. It selects a token without verifying it.
func AccessToken(r *http.Request) (string, error) {
	if headers, present := r.Header["Authorization"]; present {
		fields := strings.Fields(r.Header.Get("Authorization"))
		if len(headers) != 1 || len(fields) != 2 || !strings.EqualFold(fields[0], "Bearer") {
			return "", auth.ErrUnauthorized
		}
		return fields[1], nil
	}
	if cookie, e := r.Cookie("access_token"); e == nil && cookie.Value != "" {
		return cookie.Value, nil
	}
	return "", auth.ErrUnauthorized
}

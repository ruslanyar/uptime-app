package monitorcontroller

import (
	"net/http"
	"uptime-app/backend/internal/httpapi/request"
	"uptime-app/backend/internal/monitor"
)

// Favicon discovers a public site's icon without changing the monitor.
// @Summary Fetch a site favicon
// @Description Accepts access_token cookie or Authorization: Bearer JWT, with header precedence. Discovers HTML icon links then /favicon.ico. Only public HTTP(S) destinations are fetched, including redirects. Missing, blocked, oversized or unavailable icons return 204. No monitor data is changed.
// @Tags monitor
// @ID monitorFavicon
// @Produce image/png,image/jpeg,image/gif,image/webp,image/x-icon,image/svg+xml
// @Param url query string true "Site URL, with the same validation as monitor URLs"
// @Security BearerAuth
// @Success 200 {file} binary "Site icon"
// @Header 200 {string} Cache-Control "no-store"
// @Success 204 "Icon unavailable"
// @Failure 400 {object} response.ErrorResponse "Invalid URL"
// @Failure 401 {object} response.ErrorResponse "Invalid credentials or token"
// @Failure 403 {object} response.ErrorResponse "Origin rejected"
// @Failure 500 {object} response.ErrorResponse "Authentication service unavailable"
// @Router /monitors/favicon [get]
func (c *Controller) Favicon(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	token, err := request.AccessToken(r)
	if err != nil {
		failure(w, err)
		return
	}
	if _, err = c.auth.Me(r.Context(), token); err != nil {
		failure(w, err)
		return
	}
	raw, err := monitor.Validate(r.URL.Query().Get("url"), 60)
	if err != nil {
		failure(w, err)
		return
	}
	icon, err := c.icons.Resolve(r.Context(), raw)
	if err != nil {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	w.Header().Set("Content-Type", icon.Type)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	// SVGs must remain inert even when the resource is opened as a document.
	w.Header().Set("Content-Security-Policy", "sandbox; default-src 'none'")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(icon.Data)
}

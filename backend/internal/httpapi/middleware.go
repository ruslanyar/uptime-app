package httpapi

import (
	"net/http"
	"strings"

	"uptime-app/backend/internal/config"
	"uptime-app/backend/internal/httpapi/response"
)

func protect(cfg config.Config, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Vary", "Origin")
		origins := r.Header.Values("Origin")
		origin := r.Header.Get("Origin")
		if len(origins) > 0 && (len(origins) != 1 || !cfg.AllowedOrigins[origin]) {
			response.Error(w, 403, "forbidden_origin", "Origin is not allowed")
			return
		}
		if origin != "" {
			// Allowed clients must be able to read error responses from handlers too.
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Credentials", "true")
		}
		// Preflight grants permission only; it must not invoke application handlers.
		if r.Method == http.MethodOptions {
			w.Header().Add("Vary", "Access-Control-Request-Method")
			w.Header().Add("Vary", "Access-Control-Request-Headers")
			method := r.Header.Get("Access-Control-Request-Method")
			if origin == "" || (method != "GET" && method != "POST" && method != "PUT" && method != "DELETE") {
				response.Error(w, 403, "invalid_preflight", "Invalid preflight")
				return
			}
			for _, h := range strings.Split(r.Header.Get("Access-Control-Request-Headers"), ",") {
				switch strings.ToLower(strings.TrimSpace(h)) {
				case "", "content-type", "authorization", "x-csrf-protection":
				default:
					response.Error(w, 403, "invalid_preflight", "Header is not allowed")
					return
				}
			}
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-CSRF-Protection")
			w.WriteHeader(204)
			return
		}
		// Missing Origin must not let callers bypass the required CSRF header.
		if (r.Method == http.MethodPost || r.Method == http.MethodPut || r.Method == http.MethodDelete) && (len(r.Header.Values("X-CSRF-Protection")) != 1 || r.Header.Get("X-CSRF-Protection") != "1") {
			response.Error(w, 403, "csrf_required", "X-CSRF-Protection: 1 is required")
			return
		}
		next.ServeHTTP(w, r)
	})
}

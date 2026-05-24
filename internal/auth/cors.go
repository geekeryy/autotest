package auth

import (
	"net/http"
	"strings"
)

// CORSMiddleware sets CORS headers. When allowAll is true (development), any
// origin is permitted. Otherwise only staticOrigins from CORS_ALLOWED_ORIGINS
// are allowed. OAuth trusted frontend origins are unrelated; see frontend_origin.go.
func CORSMiddleware(staticOrigins []string, allowAll bool) func(http.Handler) http.Handler {
	static := make(map[string]struct{}, len(staticOrigins))
	for _, origin := range staticOrigins {
		if origin = strings.TrimRight(strings.TrimSpace(origin), "/"); origin != "" {
			static[origin] = struct{}{}
		}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if allowAll {
				w.Header().Set("Access-Control-Allow-Origin", "*")
			} else {
				origin := strings.TrimRight(strings.TrimSpace(r.Header.Get("Origin")), "/")
				if origin != "" {
					if _, ok := static[origin]; ok {
						w.Header().Set("Access-Control-Allow-Origin", origin)
						w.Header().Set("Vary", "Origin")
					}
				}
			}
			w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

package middleware

import (
	"net/http"
	"strconv"
	"strings"
)

// CORSConfig defines the CORS policy applied by the CORS middleware.
type CORSConfig struct {
	// Origin is the value for Access-Control-Allow-Origin.
	// Use "*" to allow any origin. Empty string disables the middleware.
	Origin string

	// Methods lists the allowed HTTP methods for preflight requests.
	Methods []string

	// Headers lists the allowed request headers for preflight requests.
	// Defaults to ["Content-Type"] when empty, because Content-Type
	// with application/json is not CORS-safelisted and would otherwise
	// silently block most JSON API requests.
	Headers []string

	// ExposedHeaders lists response headers the browser is allowed to read.
	// Empty means no extra headers are exposed.
	ExposedHeaders []string

	// MaxAge is the preflight cache duration in seconds.
	// Defaults to 86400 (24 hours) when zero.
	MaxAge int

	// Credentials sets Access-Control-Allow-Credentials to "true".
	// Must not be used with Origin "*".
	Credentials bool
}

// CORS returns a middleware that handles cross-origin requests.
// When cfg.Origin is empty the middleware is a no-op.
func CORS(cfg CORSConfig) func(http.Handler) http.Handler {
	if cfg.Origin == "" {
		return func(next http.Handler) http.Handler { return next }
	}

	methods := strings.Join(cfg.Methods, ", ")

	headers := "Content-Type"
	if len(cfg.Headers) > 0 {
		headers = strings.Join(cfg.Headers, ", ")
	}

	maxAge := "86400"
	if cfg.MaxAge > 0 {
		maxAge = strconv.Itoa(cfg.MaxAge)
	}

	exposed := strings.Join(cfg.ExposedHeaders, ", ")

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Access-Control-Allow-Origin", cfg.Origin)

			if cfg.Origin != "*" {
				w.Header().Add("Vary", "Origin")
			}

			if cfg.Credentials {
				w.Header().Set("Access-Control-Allow-Credentials", "true")
			}

			if exposed != "" {
				w.Header().Set("Access-Control-Expose-Headers", exposed)
			}

			if r.Method == http.MethodOptions {
				if methods != "" {
					w.Header().Set("Access-Control-Allow-Methods", methods)
				}
				w.Header().Set("Access-Control-Allow-Headers", headers)
				w.Header().Set("Access-Control-Max-Age", maxAge)
				w.WriteHeader(http.StatusNoContent)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

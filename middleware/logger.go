package middleware

import (
	"log/slog"
	"net/http"
	"time"
)

// Logger middleware using slog
func Logger(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		fn := func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			// Call the next handler
			next.ServeHTTP(w, r)

			// Log request details
			logger.Info("request",
				"method", r.Method,
				"path", r.URL.Path,
				"remote", r.RemoteAddr,
				"duration", time.Since(start).String(),
			)
		}
		return http.HandlerFunc(fn)
	}
}

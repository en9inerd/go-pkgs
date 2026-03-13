package middleware

import (
	"log/slog"
	"net"
	"net/http"
	"time"
)

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

// Logger middleware logs each request with method, path, client IP,
// response status code, and duration.
func Logger(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
			start := time.Now()

			next.ServeHTTP(sw, r)

			duration := time.Since(start)

			// RemoteAddr may be a bare IP (no port) if RealIP middleware
			// already processed the request, or host:port from the stdlib.
			remoteIP := r.RemoteAddr
			if host, _, err := net.SplitHostPort(remoteIP); err == nil {
				remoteIP = host
			}

			logger.Info("http request",
				"method", r.Method,
				"path", r.URL.Path,
				"ip", remoteIP,
				"status", sw.status,
				"duration", duration,
			)
		})
	}
}

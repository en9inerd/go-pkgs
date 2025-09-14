package middleware

import (
	"log/slog"
	"net/http"
)

// SizeLimit middleware rejects requests with bodies larger than size.
func SizeLimit(size int64, logger *slog.Logger) func(http.Handler) http.Handler {
	return func(h http.Handler) http.Handler {
		fn := func(w http.ResponseWriter, r *http.Request) {
			// Check Content-Length if provided
			if r.ContentLength > size {
				http.Error(w, "request too large", http.StatusRequestEntityTooLarge)
				return
			}

			// Limit body reader directly (streaming, avoids full buffering)
			r.Body = http.MaxBytesReader(w, r.Body, size)

			h.ServeHTTP(w, r)
		}
		return http.HandlerFunc(fn)
	}
}

package middleware

import (
	"net/http"
)

// SizeLimit middleware rejects requests with bodies larger than size.
func SizeLimit(size int64) func(http.Handler) http.Handler {
	return func(h http.Handler) http.Handler {
		fn := func(w http.ResponseWriter, r *http.Request) {
			if r.ContentLength > size {
				http.Error(w, "request too large", http.StatusRequestEntityTooLarge)
				return
			}

			r.Body = http.MaxBytesReader(w, r.Body, size)

			h.ServeHTTP(w, r)
		}
		return http.HandlerFunc(fn)
	}
}

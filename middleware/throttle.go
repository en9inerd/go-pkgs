package middleware

import (
	"net/http"
)

// GlobalThrottle returns a middleware that limits the total number
// of in-flight requests across all routes in the server.
func GlobalThrottle(limit int64) func(http.Handler) http.Handler {
	if limit <= 0 {
		// no throttling
		return func(h http.Handler) http.Handler { return h }
	}

	// one global semaphore shared by all handlers
	ch := make(chan struct{}, limit)

	return func(h http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			select {
			case ch <- struct{}{}: // acquired slot
				defer func() { <-ch }()
				h.ServeHTTP(w, r)
			default: // no slot available
				http.Error(w, "too many requests", http.StatusTooManyRequests)
			}
		})
	}
}

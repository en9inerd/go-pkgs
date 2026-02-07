package middleware

import (
	"net/http"
)

// ThrottleConfig holds configuration for the throttle middleware
type ThrottleConfig struct {
	Limit   int64
	Message string
}

// GlobalThrottle returns a middleware that limits the total number
// of in-flight requests across all routes in the server.
func GlobalThrottle(limit int64) func(http.Handler) http.Handler {
	return GlobalThrottleWithConfig(ThrottleConfig{
		Limit:   limit,
		Message: "too many requests",
	})
}

// GlobalThrottleWithConfig returns a throttle middleware with custom configuration.
func GlobalThrottleWithConfig(cfg ThrottleConfig) func(http.Handler) http.Handler {
	if cfg.Limit <= 0 {
		// no throttling
		return func(h http.Handler) http.Handler { return h }
	}

	if cfg.Message == "" {
		cfg.Message = "too many requests"
	}

	// one global semaphore shared by all handlers
	ch := make(chan struct{}, cfg.Limit)

	return func(h http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			select {
			case ch <- struct{}{}: // acquired slot
				defer func() { <-ch }()
				h.ServeHTTP(w, r)
			default: // no slot available
				http.Error(w, cfg.Message, http.StatusTooManyRequests)
			}
		})
	}
}

package middleware

import (
	"net/http"
	"time"
)

// Timeout creates a timeout middleware with the default message "Request timeout"
func Timeout(timeout time.Duration) func(http.Handler) http.Handler {
	return TimeoutWithMessage(timeout, "Request timeout")
}

// TimeoutWithMessage creates a timeout middleware with a custom message
func TimeoutWithMessage(timeout time.Duration, message string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.TimeoutHandler(next, timeout, message)
	}
}

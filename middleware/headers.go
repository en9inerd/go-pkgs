package middleware

import (
	"net/http"
	"strings"
)

// Headers middleware adds headers to response.
// Header values are sanitized to prevent HTTP header injection attacks.
func Headers(headers ...string) func(http.Handler) http.Handler {
	return func(h http.Handler) http.Handler {
		fn := func(w http.ResponseWriter, r *http.Request) {
			for _, h := range headers {
				elems := strings.SplitN(h, ":", 2)
				if len(elems) != 2 {
					continue
				}
				key := strings.TrimSpace(elems[0])
				value := strings.TrimSpace(elems[1])

				if strings.ContainsAny(value, "\r\n") {
					continue
				}
				w.Header().Set(key, value)
			}
			h.ServeHTTP(w, r)
		}
		return http.HandlerFunc(fn)
	}
}

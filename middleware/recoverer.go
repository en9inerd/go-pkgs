package middleware

import (
	"log/slog"
	"net/http"
	"runtime/debug"
)

// Recoverer is a middleware that recovers from panics, logs the panic and returns a HTTP 500 status if possible.
func Recoverer(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(h http.Handler) http.Handler {
		fn := func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if rvr := recover(); rvr != nil {
					// Log panic with request context
					logger.Error("panic recovered",
						slog.Any("panic", rvr),
						slog.String("url", r.URL.String()),
						slog.String("remote_addr", r.RemoteAddr),
						slog.String("stack", string(debug.Stack())),
					)

					// Only send 500 if we can still write a response
					if rvr != http.ErrAbortHandler {
						http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
					}
				}
			}()
			h.ServeHTTP(w, r)
		}
		return http.HandlerFunc(fn)
	}
}

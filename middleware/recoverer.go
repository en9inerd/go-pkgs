package middleware

import (
	"log/slog"
	"net/http"
	"runtime/debug"
)

// recoverWriter wraps a ResponseWriter to track whether headers have been sent.
type recoverWriter struct {
	http.ResponseWriter
	wroteHeader bool
}

func (w *recoverWriter) WriteHeader(code int) {
	w.wroteHeader = true
	w.ResponseWriter.WriteHeader(code)
}

func (w *recoverWriter) Write(b []byte) (int, error) {
	w.wroteHeader = true
	return w.ResponseWriter.Write(b)
}

func (w *recoverWriter) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}

// Recoverer is a middleware that recovers from panics, logs the panic and returns a HTTP 500 status if possible.
// If includeStack is true, full stack traces are logged. In production, set includeStack to false to prevent
// information disclosure if logs are exposed.
func Recoverer(logger *slog.Logger, includeStack bool) func(http.Handler) http.Handler {
	return func(h http.Handler) http.Handler {
		fn := func(w http.ResponseWriter, r *http.Request) {
			rw := &recoverWriter{ResponseWriter: w}
			defer func() {
				if rvr := recover(); rvr != nil {
					attrs := []any{
						slog.Any("panic", rvr),
						slog.String("url", r.URL.String()),
						slog.String("remote_addr", r.RemoteAddr),
					}

					if includeStack {
						attrs = append(attrs, slog.String("stack", string(debug.Stack())))
					}

					logger.Error("panic recovered", attrs...)

					// Only send 500 if we can still write a response
					if rvr != http.ErrAbortHandler && !rw.wroteHeader {
						http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
					}
				}
			}()
			h.ServeHTTP(rw, r)
		}
		return http.HandlerFunc(fn)
	}
}

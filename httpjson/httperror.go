package httpjson

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"runtime"
	"strings"
)

// Logger wraps slog.Logger for error reporting
type Logger struct {
	l *slog.Logger
}

// NewLogger creates a new error logger with slog.Logger
func NewLogger(l *slog.Logger) *Logger {
	return &Logger{l: l}
}

// Respond logs the error and sends a JSON error response
func (e *Logger) Respond(w http.ResponseWriter, r *http.Request, httpCode int, err error, msg ...string) {
	m := strings.Join(msg, ". ")
	if e.l != nil {
		e.l.Error(errDetails(r, httpCode, err, m))
	}
	writeJSONWithStatus(w, JSON{"error": m}, httpCode)
}

// RespondJSON logs the error with slog and sends a JSON error response
func SendErrorJSON(w http.ResponseWriter, r *http.Request, l *slog.Logger, code int, err error, msg string) {
	if l != nil {
		l.Error(errDetails(r, code, err, msg))
	}
	writeJSONWithStatus(w, JSON{"error": msg}, code)
}

func errDetails(r *http.Request, code int, err error, msg string) string {
	q := r.URL.String()
	if qun, e := url.QueryUnescape(q); e == nil {
		q = qun
	}

	srcFileInfo := ""
	if pc, file, line, ok := runtime.Caller(2); ok {
		fnameElems := strings.Split(file, "/")
		funcNameElems := strings.Split(runtime.FuncForPC(pc).Name(), "/")
		srcFileInfo = fmt.Sprintf(" [caused by %s:%d %s]",
			strings.Join(fnameElems[len(fnameElems)-3:], "/"),
			line, funcNameElems[len(funcNameElems)-1])
	}

	remoteIP := r.RemoteAddr
	if pos := strings.Index(remoteIP, ":"); pos >= 0 {
		remoteIP = remoteIP[:pos]
	}
	if err == nil {
		err = errors.New("no error")
	}
	return fmt.Sprintf("%s - %v - %d - %s - %s%s", msg, err, code, remoteIP, q, srcFileInfo)
}

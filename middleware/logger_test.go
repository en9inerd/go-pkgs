package middleware

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
)

type logEntry struct {
	Method string
	Path   string
	IP     string
	Status int
}

func captureLogger() (*slog.Logger, *[]logEntry) {
	var entries []logEntry
	handler := &captureHandler{entries: &entries}
	return slog.New(handler), &entries
}

type captureHandler struct {
	entries *[]logEntry
}

func (h *captureHandler) Enabled(_ context.Context, _ slog.Level) bool { return true }
func (h *captureHandler) WithAttrs(_ []slog.Attr) slog.Handler         { return h }
func (h *captureHandler) WithGroup(_ string) slog.Handler              { return h }
func (h *captureHandler) Handle(_ context.Context, r slog.Record) error {
	e := logEntry{}
	r.Attrs(func(a slog.Attr) bool {
		switch a.Key {
		case "method":
			e.Method = a.Value.String()
		case "path":
			e.Path = a.Value.String()
		case "ip":
			e.IP = a.Value.String()
		case "status":
			e.Status = int(a.Value.Int64())
		}
		return true
	})
	*h.entries = append(*h.entries, e)
	return nil
}

func TestLogger_HostPort(t *testing.T) {
	logger, entries := captureLogger()

	handler := Logger(logger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "192.168.1.1:12345"
	handler.ServeHTTP(httptest.NewRecorder(), req)

	if len(*entries) != 1 {
		t.Fatalf("expected 1 log entry, got %d", len(*entries))
	}
	e := (*entries)[0]
	if e.IP != "192.168.1.1" {
		t.Errorf("IP = %q, want %q", e.IP, "192.168.1.1")
	}
	if e.Method != "GET" {
		t.Errorf("Method = %q, want %q", e.Method, "GET")
	}
	if e.Path != "/test" {
		t.Errorf("Path = %q, want %q", e.Path, "/test")
	}
	if e.Status != 200 {
		t.Errorf("Status = %d, want 200", e.Status)
	}
}

func TestLogger_BareIP(t *testing.T) {
	logger, entries := captureLogger()

	handler := Logger(logger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))

	req := httptest.NewRequest("POST", "/api", nil)
	req.RemoteAddr = "10.0.0.1"
	handler.ServeHTTP(httptest.NewRecorder(), req)

	e := (*entries)[0]
	if e.IP != "10.0.0.1" {
		t.Errorf("IP = %q, want %q", e.IP, "10.0.0.1")
	}
	if e.Status != 404 {
		t.Errorf("Status = %d, want 404", e.Status)
	}
}

func TestLogger_IPv6(t *testing.T) {
	logger, entries := captureLogger()

	handler := Logger(logger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))

	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "[::1]:8080"
	handler.ServeHTTP(httptest.NewRecorder(), req)

	e := (*entries)[0]
	if e.IP != "::1" {
		t.Errorf("IP = %q, want %q", e.IP, "::1")
	}
}

func TestLogger_DefaultStatus(t *testing.T) {
	logger, entries := captureLogger()

	handler := Logger(logger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok"))
	}))

	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "1.2.3.4:5678"
	handler.ServeHTTP(httptest.NewRecorder(), req)

	e := (*entries)[0]
	if e.Status != 200 {
		t.Errorf("Status = %d, want 200 (implicit)", e.Status)
	}
}

func TestLogger_CapturesExplicitStatus(t *testing.T) {
	logger, entries := captureLogger()

	handler := Logger(logger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
	}))

	req := httptest.NewRequest("POST", "/create", nil)
	req.RemoteAddr = "10.0.0.1"
	handler.ServeHTTP(httptest.NewRecorder(), req)

	e := (*entries)[0]
	if e.Status != 201 {
		t.Errorf("Status = %d, want 201", e.Status)
	}
}

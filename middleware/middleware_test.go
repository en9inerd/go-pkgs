package middleware

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

// --------------- Recoverer ---------------

func TestRecoverer_PanicReturns500(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	handler := Recoverer(logger, false)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("boom")
	}))

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, httptest.NewRequest("GET", "/", nil))

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", w.Code)
	}
}

func TestRecoverer_NoPanic(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	handler := Recoverer(logger, false)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, httptest.NewRequest("GET", "/", nil))

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}
}

func TestRecoverer_WithStack(t *testing.T) {
	var buf strings.Builder
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelError}))
	handler := Recoverer(logger, true)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("with stack")
	}))

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, httptest.NewRequest("GET", "/test", nil))

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", w.Code)
	}
	if !strings.Contains(buf.String(), "stack=") {
		t.Error("expected stack trace in log output")
	}
}

func TestRecoverer_ErrAbortHandler(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	handler := Recoverer(logger, false)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic(http.ErrAbortHandler)
	}))

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, httptest.NewRequest("GET", "/", nil))

	// ErrAbortHandler should not write a 500 response
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200 (no response written for ErrAbortHandler)", w.Code)
	}
}

func TestRecoverer_PanicAfterWrite(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	handler := Recoverer(logger, false)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("partial"))
		panic("oops after write")
	}))

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, httptest.NewRequest("GET", "/", nil))

	// Should NOT append error text when headers already sent
	body := w.Body.String()
	if strings.Contains(body, "Internal Server Error") {
		t.Error("recoverer should not write error after headers already sent")
	}
	if body != "partial" {
		t.Errorf("body = %q, want %q", body, "partial")
	}
}

func TestLogger_StatusWriterUnwrap(t *testing.T) {
	sw := &statusWriter{ResponseWriter: httptest.NewRecorder(), status: http.StatusOK}
	if sw.Unwrap() == nil {
		t.Error("Unwrap() should return the underlying ResponseWriter")
	}
}

// --------------- GlobalThrottle ---------------

func TestGlobalThrottle_AllowsUnderLimit(t *testing.T) {
	handler := GlobalThrottle(10)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, httptest.NewRequest("GET", "/", nil))

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}
}

func TestGlobalThrottle_RejectsOverLimit(t *testing.T) {
	blocker := make(chan struct{})
	handler := GlobalThrottle(1)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-blocker
		w.WriteHeader(http.StatusOK)
	}))

	// First request blocks
	var wg sync.WaitGroup
	wg.Go(func() {
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, httptest.NewRequest("GET", "/", nil))
	})

	// Give first request time to acquire the slot
	time.Sleep(20 * time.Millisecond)

	// Second request should be rejected
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, httptest.NewRequest("GET", "/", nil))

	if w.Code != http.StatusTooManyRequests {
		t.Errorf("status = %d, want 429", w.Code)
	}

	close(blocker)
	wg.Wait()
}

func TestGlobalThrottle_ZeroLimitNoOp(t *testing.T) {
	handler := GlobalThrottle(0)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, httptest.NewRequest("GET", "/", nil))

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200 (no throttle for limit=0)", w.Code)
	}
}

func TestGlobalThrottleWithConfig_CustomMessage(t *testing.T) {
	blocker := make(chan struct{})
	handler := GlobalThrottleWithConfig(ThrottleConfig{
		Limit:   1,
		Message: "slow down",
	})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-blocker
	}))

	go func() {
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, httptest.NewRequest("GET", "/", nil))
	}()

	time.Sleep(20 * time.Millisecond)

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, httptest.NewRequest("GET", "/", nil))

	if !strings.Contains(w.Body.String(), "slow down") {
		t.Errorf("body = %q, want 'slow down'", w.Body.String())
	}

	close(blocker)
}

// --------------- SizeLimit ---------------

func TestSizeLimit_AllowsSmallBody(t *testing.T) {
	handler := SizeLimit(1024)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("POST", "/", strings.NewReader("small"))
	req.ContentLength = 5
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}
}

func TestSizeLimit_RejectsLargeContentLength(t *testing.T) {
	handler := SizeLimit(10)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("POST", "/", strings.NewReader(strings.Repeat("x", 100)))
	req.ContentLength = 100
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("status = %d, want 413", w.Code)
	}
}

func TestSizeLimit_MaxBytesReaderEnforced(t *testing.T) {
	handler := SizeLimit(10)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "too large", http.StatusRequestEntityTooLarge)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))

	// ContentLength = -1 (unknown), but body is large
	req := httptest.NewRequest("POST", "/", strings.NewReader(strings.Repeat("x", 100)))
	req.ContentLength = -1
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("status = %d, want 413 (MaxBytesReader)", w.Code)
	}
}

// --------------- Timeout ---------------

func TestTimeout_CompletesInTime(t *testing.T) {
	handler := Timeout(1 * time.Second)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok"))
	}))

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, httptest.NewRequest("GET", "/", nil))

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}
	if w.Body.String() != "ok" {
		t.Errorf("body = %q", w.Body.String())
	}
}

func TestTimeout_ExceedsDeadline(t *testing.T) {
	handler := Timeout(10 * time.Millisecond)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-time.After(5 * time.Second):
		}
	}))

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, httptest.NewRequest("GET", "/", nil))

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want 503", w.Code)
	}
}

func TestTimeoutWithMessage_CustomMessage(t *testing.T) {
	handler := TimeoutWithMessage(10*time.Millisecond, "timed out")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-time.After(5 * time.Second):
		}
	}))

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, httptest.NewRequest("GET", "/", nil))

	if !strings.Contains(w.Body.String(), "timed out") {
		t.Errorf("body = %q, want 'timed out'", w.Body.String())
	}
}

// --------------- Headers ---------------

func TestHeaders_SetsHeaders(t *testing.T) {
	handler := Headers(
		"X-Content-Type-Options: nosniff",
		"X-Frame-Options: DENY",
	)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, httptest.NewRequest("GET", "/", nil))

	if w.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Errorf("X-Content-Type-Options = %q", w.Header().Get("X-Content-Type-Options"))
	}
	if w.Header().Get("X-Frame-Options") != "DENY" {
		t.Errorf("X-Frame-Options = %q", w.Header().Get("X-Frame-Options"))
	}
}

func TestHeaders_SkipsInvalidFormat(t *testing.T) {
	handler := Headers(
		"no-colon-here",
		"Valid: header",
	)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, httptest.NewRequest("GET", "/", nil))

	if w.Header().Get("Valid") != "header" {
		t.Errorf("Valid = %q", w.Header().Get("Valid"))
	}
}

func TestHeaders_BlocksHeaderInjection(t *testing.T) {
	handler := Headers(
		"X-Bad: value\r\nInjected: header",
	)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, httptest.NewRequest("GET", "/", nil))

	if w.Header().Get("X-Bad") != "" {
		t.Error("header with CRLF should be skipped")
	}
	if w.Header().Get("Injected") != "" {
		t.Error("injected header should not exist")
	}
}

func TestHeaders_ValueWithColon(t *testing.T) {
	handler := Headers(
		"Content-Security-Policy: default-src 'self'; script-src 'self'",
	)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, httptest.NewRequest("GET", "/", nil))

	got := w.Header().Get("Content-Security-Policy")
	if got != "default-src 'self'; script-src 'self'" {
		t.Errorf("CSP = %q", got)
	}
}

// --------------- Health ---------------

func TestHealth_ReturnsOK(t *testing.T) {
	handler := Health(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("next handler should not be called for /health")
	}))

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, httptest.NewRequest("GET", "/health", nil))

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}

	var resp HealthResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Status != "ok" {
		t.Errorf("status = %q, want %q", resp.Status, "ok")
	}

	ct := w.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("Content-Type = %q", ct)
	}
}

func TestHealth_PassesThroughOtherPaths(t *testing.T) {
	called := false
	handler := Health(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, httptest.NewRequest("GET", "/api/data", nil))

	if !called {
		t.Error("next handler should be called for non-health path")
	}
}

func TestHealth_IgnoresPostMethod(t *testing.T) {
	called := false
	handler := Health(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	}))

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, httptest.NewRequest("POST", "/health", nil))

	if !called {
		t.Error("POST /health should pass through to next handler")
	}
}

// --------------- RealIP ---------------

func TestRealIP_XForwardedFor(t *testing.T) {
	var gotAddr string
	handler := RealIP(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAddr = r.RemoteAddr
	}))

	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "10.0.0.1:1234"
	req.Header.Set("X-Forwarded-For", "203.0.113.50, 70.41.3.18")
	handler.ServeHTTP(httptest.NewRecorder(), req)

	if gotAddr != "70.41.3.18" {
		t.Errorf("RemoteAddr = %q, want %q", gotAddr, "70.41.3.18")
	}
}

func TestRealIP_XRealIP(t *testing.T) {
	var gotAddr string
	handler := RealIP(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAddr = r.RemoteAddr
	}))

	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "10.0.0.1:1234"
	req.Header.Set("X-Real-IP", "203.0.113.99")
	handler.ServeHTTP(httptest.NewRecorder(), req)

	if gotAddr != "203.0.113.99" {
		t.Errorf("RemoteAddr = %q, want %q", gotAddr, "203.0.113.99")
	}
}

func TestRealIP_NoHeaders(t *testing.T) {
	var gotAddr string
	handler := RealIP(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAddr = r.RemoteAddr
	}))

	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "192.0.2.1:5678"
	handler.ServeHTTP(httptest.NewRecorder(), req)

	if gotAddr != "192.0.2.1" {
		t.Errorf("RemoteAddr = %q, want %q", gotAddr, "192.0.2.1")
	}
}

func TestRealIPWithTrustedProxies_Trusted(t *testing.T) {
	var gotAddr string
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAddr = r.RemoteAddr
	})
	handler := RealIPWithTrustedProxies([]string{"10.0.0.1"}, inner)

	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "10.0.0.1:1234"
	req.Header.Set("X-Forwarded-For", "203.0.113.50")
	handler.ServeHTTP(httptest.NewRecorder(), req)

	if gotAddr != "203.0.113.50" {
		t.Errorf("RemoteAddr = %q, want %q", gotAddr, "203.0.113.50")
	}
}

func TestRealIPWithTrustedProxies_Untrusted(t *testing.T) {
	var gotAddr string
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAddr = r.RemoteAddr
	})
	handler := RealIPWithTrustedProxies([]string{"10.0.0.99"}, inner)

	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "192.0.2.1:1234"
	req.Header.Set("X-Forwarded-For", "203.0.113.50")
	handler.ServeHTTP(httptest.NewRecorder(), req)

	// Untrusted proxy - header should be ignored
	if gotAddr != "192.0.2.1:1234" {
		t.Errorf("RemoteAddr = %q, want %q (untouched)", gotAddr, "192.0.2.1:1234")
	}
}

func TestRealIPWithTrustedProxies_CIDR(t *testing.T) {
	var gotAddr string
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAddr = r.RemoteAddr
	})
	handler := RealIPWithTrustedProxies([]string{"10.0.0.0/8"}, inner)

	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "10.0.0.5:1234"
	req.Header.Set("X-Real-IP", "203.0.113.77")
	handler.ServeHTTP(httptest.NewRecorder(), req)

	if gotAddr != "203.0.113.77" {
		t.Errorf("RemoteAddr = %q, want %q", gotAddr, "203.0.113.77")
	}
}

func TestRealIPWithTrustedProxies_NilDefaultsToPrivate(t *testing.T) {
	var gotAddr string
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAddr = r.RemoteAddr
	})
	handler := RealIPWithTrustedProxies(nil, inner)

	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "192.168.1.1:1234"
	req.Header.Set("X-Forwarded-For", "203.0.113.50")
	handler.ServeHTTP(httptest.NewRecorder(), req)

	if gotAddr != "203.0.113.50" {
		t.Errorf("RemoteAddr = %q, want %q (private IP trusted by default)", gotAddr, "203.0.113.50")
	}
}

// --------------- StripSlashes ---------------

func TestStripSlashes_RemovesTrailing(t *testing.T) {
	var gotPath string
	handler := StripSlashes(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
	}))

	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/api/test/", nil))

	if gotPath != "/api/test" {
		t.Errorf("path = %q, want %q", gotPath, "/api/test")
	}
}

func TestStripSlashes_PreservesRoot(t *testing.T) {
	var gotPath string
	handler := StripSlashes(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
	}))

	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/", nil))

	if gotPath != "/" {
		t.Errorf("path = %q, want %q (root preserved)", gotPath, "/")
	}
}

func TestStripSlashes_NoTrailingSlash(t *testing.T) {
	var gotPath string
	handler := StripSlashes(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
	}))

	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/api/test", nil))

	if gotPath != "/api/test" {
		t.Errorf("path = %q, want %q (unchanged)", gotPath, "/api/test")
	}
}

// --------------- Timeout (context cancellation) ---------------

func TestTimeout_ContextCancelled(t *testing.T) {
	handler := Timeout(50 * time.Millisecond)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))

	ctx, cancel := context.WithCancel(context.Background())
	req := httptest.NewRequest("GET", "/", nil).WithContext(ctx)

	go func() {
		time.Sleep(10 * time.Millisecond)
		cancel()
	}()

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable && w.Code != http.StatusOK {
		t.Errorf("status = %d", w.Code)
	}
}

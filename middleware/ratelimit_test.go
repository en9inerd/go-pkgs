package middleware

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

func TestRateLimit_AllowsUnderLimit(t *testing.T) {
	handler := RateLimit(RateLimitConfig{RPS: 10, Burst: 10})(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}),
	)

	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "192.0.2.1:1234"
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}
}

func TestRateLimit_RejectsOverBurst(t *testing.T) {
	handler := RateLimit(RateLimitConfig{RPS: 1, Burst: 2})(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}),
	)

	for i := range 2 {
		req := httptest.NewRequest("GET", "/", nil)
		req.RemoteAddr = "192.0.2.1:1234"
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Errorf("request %d: status = %d, want 200", i, w.Code)
		}
	}

	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "192.0.2.1:1234"
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusTooManyRequests {
		t.Errorf("status = %d, want 429", w.Code)
	}
}

func TestRateLimit_SeparateIPsIndependent(t *testing.T) {
	handler := RateLimit(RateLimitConfig{RPS: 1, Burst: 1})(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}),
	)

	req1 := httptest.NewRequest("GET", "/", nil)
	req1.RemoteAddr = "192.0.2.1:1234"
	w1 := httptest.NewRecorder()
	handler.ServeHTTP(w1, req1)
	if w1.Code != http.StatusOK {
		t.Fatalf("IP1 first request: status = %d, want 200", w1.Code)
	}

	req2 := httptest.NewRequest("GET", "/", nil)
	req2.RemoteAddr = "192.0.2.2:1234"
	w2 := httptest.NewRecorder()
	handler.ServeHTTP(w2, req2)
	if w2.Code != http.StatusOK {
		t.Errorf("IP2: status = %d, want 200", w2.Code)
	}
}

func TestRateLimit_BareIPAddress(t *testing.T) {
	handler := RateLimit(RateLimitConfig{RPS: 1, Burst: 1})(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}),
	)

	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "192.0.2.1"
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}

	w2 := httptest.NewRecorder()
	handler.ServeHTTP(w2, req)
	if w2.Code != http.StatusTooManyRequests {
		t.Errorf("second request: status = %d, want 429", w2.Code)
	}
}

func TestRateLimit_DefaultBurst(t *testing.T) {
	handler := RateLimit(RateLimitConfig{RPS: 5})(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}),
	)

	for i := range 5 {
		req := httptest.NewRequest("GET", "/", nil)
		req.RemoteAddr = "192.0.2.1:1234"
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Errorf("request %d: status = %d, want 200", i, w.Code)
		}
	}

	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "192.0.2.1:1234"
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusTooManyRequests {
		t.Errorf("over default burst: status = %d, want 429", w.Code)
	}
}

func TestRateLimit_Refills(t *testing.T) {
	handler := RateLimit(RateLimitConfig{RPS: 100, Burst: 1})(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}),
	)

	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "192.0.2.1:1234"

	w1 := httptest.NewRecorder()
	handler.ServeHTTP(w1, req)
	if w1.Code != http.StatusOK {
		t.Fatalf("first request: status = %d, want 200", w1.Code)
	}

	w2 := httptest.NewRecorder()
	handler.ServeHTTP(w2, req)
	if w2.Code != http.StatusTooManyRequests {
		t.Fatalf("exhausted: status = %d, want 429", w2.Code)
	}

	time.Sleep(20 * time.Millisecond)

	w3 := httptest.NewRecorder()
	handler.ServeHTTP(w3, req)
	if w3.Code != http.StatusOK {
		t.Errorf("after refill: status = %d, want 200", w3.Code)
	}
}

func TestRateLimit_ConcurrentAccess(t *testing.T) {
	handler := RateLimit(RateLimitConfig{RPS: 10000, Burst: 10000})(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}),
	)

	var wg sync.WaitGroup
	for range 100 {
		wg.Go(func() {
			req := httptest.NewRequest("GET", "/", nil)
			req.RemoteAddr = "192.0.2.1:1234"
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)
		})
	}
	wg.Wait()
}

func TestExtractIP_HostPort(t *testing.T) {
	if got := extractIP("192.0.2.1:8080"); got != "192.0.2.1" {
		t.Errorf("extractIP(host:port) = %q, want %q", got, "192.0.2.1")
	}
}

func TestExtractIP_BareIP(t *testing.T) {
	if got := extractIP("192.0.2.1"); got != "192.0.2.1" {
		t.Errorf("extractIP(bare) = %q, want %q", got, "192.0.2.1")
	}
}

func TestExtractIP_IPv6(t *testing.T) {
	if got := extractIP("[::1]:8080"); got != "::1" {
		t.Errorf("extractIP(ipv6) = %q, want %q", got, "::1")
	}
}

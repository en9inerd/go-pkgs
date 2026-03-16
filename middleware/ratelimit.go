package middleware

import (
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/en9inerd/go-pkgs/ratelimit"
)

// RateLimitConfig configures the per-IP rate limiting middleware.
type RateLimitConfig struct {
	// RPS is the sustained requests per second allowed per IP address.
	RPS float64
	// Burst is the maximum number of requests allowed in a burst above the
	// sustained rate. When zero, defaults to max(1, int(RPS)).
	Burst int
}

type ipEntry struct {
	bucket   *ratelimit.TokenBucket
	lastSeen time.Time
}

type ipStore struct {
	mu      sync.Mutex
	entries map[string]*ipEntry
	rps     float64
	burst   float64
}

func (s *ipStore) allow(ip string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	e, ok := s.entries[ip]
	if !ok {
		e = &ipEntry{bucket: ratelimit.NewTokenBucket(s.burst, s.rps)}
		s.entries[ip] = e
	}
	e.lastSeen = time.Now()
	return e.bucket.Allow()
}

func (s *ipStore) cleanup() {
	t := time.NewTicker(time.Minute)
	defer t.Stop()
	for range t.C {
		s.mu.Lock()
		for ip, e := range s.entries {
			if time.Since(e.lastSeen) > 3*time.Minute {
				delete(s.entries, ip)
			}
		}
		s.mu.Unlock()
	}
}

// extractIP handles both "host:port" and bare IP formats. The latter appears
// after the RealIP middleware strips the port.
func extractIP(addr string) string {
	if host, _, err := net.SplitHostPort(addr); err == nil {
		return host
	}
	return addr
}

// RateLimit returns middleware that enforces per-IP rate limiting using a token
// bucket algorithm. Each unique client IP gets its own bucket. Stale entries
// are cleaned up automatically.
func RateLimit(cfg RateLimitConfig) func(http.Handler) http.Handler {
	burst := cfg.Burst
	if burst <= 0 {
		burst = max(1, int(cfg.RPS))
	}

	store := &ipStore{
		entries: make(map[string]*ipEntry),
		rps:     cfg.RPS,
		burst:   float64(burst),
	}
	go store.cleanup()

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := extractIP(r.RemoteAddr)
			if !store.allow(ip) {
				http.Error(w, http.StatusText(http.StatusTooManyRequests), http.StatusTooManyRequests)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

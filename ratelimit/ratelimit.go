// Package ratelimit provides rate limiting utilities for API clients
package ratelimit

import (
	"context"
	"sync"
	"time"
)

// Limiter provides rate limiting functionality
type Limiter interface {
	// Wait blocks until the limiter allows the request
	Wait(ctx context.Context) error
	// Allow checks if a request is allowed without blocking
	Allow() bool
}

// TokenBucket implements a token bucket rate limiter
type TokenBucket struct {
	mu          sync.Mutex
	tokens      float64
	capacity    float64
	refillRate  float64 // tokens per second
	lastRefill  time.Time
}

// NewTokenBucket creates a new token bucket limiter
// capacity: maximum number of tokens
// refillRate: tokens added per second
func NewTokenBucket(capacity float64, refillRate float64) *TokenBucket {
	return &TokenBucket{
		tokens:     capacity,
		capacity:   capacity,
		refillRate: refillRate,
		lastRefill: time.Now(),
	}
}

// refill adds tokens based on elapsed time
func (tb *TokenBucket) refill() {
	now := time.Now()
	elapsed := now.Sub(tb.lastRefill).Seconds()
	tokensToAdd := elapsed * tb.refillRate
	tb.tokens = min(tb.capacity, tb.tokens+tokensToAdd)
	tb.lastRefill = now
}

// Allow checks if a request is allowed without blocking
func (tb *TokenBucket) Allow() bool {
	tb.mu.Lock()
	defer tb.mu.Unlock()

	tb.refill()
	if tb.tokens >= 1.0 {
		tb.tokens -= 1.0
		return true
	}
	return false
}

// Wait blocks until a token is available or context is cancelled
func (tb *TokenBucket) Wait(ctx context.Context) error {
	for {
		if tb.Allow() {
			return nil
		}

		tb.mu.Lock()
		tb.refill()
		needed := 1.0 - tb.tokens
		waitTime := time.Duration(needed/tb.refillRate) * time.Second
		tb.mu.Unlock()

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(waitTime):
			// Continue loop to try again
		}
	}
}

// FixedWindow implements a fixed window rate limiter
type FixedWindow struct {
	mu       sync.Mutex
	limit    int
	count    int
	window   time.Duration
	windowStart time.Time
}

// NewFixedWindow creates a new fixed window rate limiter
// limit: maximum requests per window
// window: time window duration
func NewFixedWindow(limit int, window time.Duration) *FixedWindow {
	return &FixedWindow{
		limit:       limit,
		window:      window,
		windowStart: time.Now(),
	}
}

// Allow checks if a request is allowed without blocking
func (fw *FixedWindow) Allow() bool {
	fw.mu.Lock()
	defer fw.mu.Unlock()

	now := time.Now()
	if now.Sub(fw.windowStart) >= fw.window {
		fw.count = 0
		fw.windowStart = now
	}

	if fw.count < fw.limit {
		fw.count++
		return true
	}
	return false
}

// Wait blocks until a request is allowed or context is cancelled
func (fw *FixedWindow) Wait(ctx context.Context) error {
	for {
		if fw.Allow() {
			return nil
		}

		fw.mu.Lock()
		now := time.Now()
		elapsed := now.Sub(fw.windowStart)
		waitTime := fw.window - elapsed
		fw.mu.Unlock()

		if waitTime <= 0 {
			continue
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(waitTime):
			// Continue loop to try again
		}
	}
}

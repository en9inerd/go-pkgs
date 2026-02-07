// Package retry provides utilities for retrying operations with configurable strategies
package retry

import (
	"context"
	"errors"
	"fmt"
	"math"
	"math/rand"
	"time"
)

// Strategy defines a retry strategy
type Strategy struct {
	MaxAttempts     int
	InitialDelay    time.Duration
	MaxDelay        time.Duration
	Multiplier      float64
	Jitter          bool
	RetryableErrors func(error) bool
}

// DefaultStrategy returns a default retry strategy with exponential backoff
func DefaultStrategy() *Strategy {
	return &Strategy{
		MaxAttempts:  3,
		InitialDelay: 100 * time.Millisecond,
		MaxDelay:     10 * time.Second,
		Multiplier:   2.0,
		Jitter:       true,
		RetryableErrors: func(err error) bool {
			// By default, retry all errors
			return true
		},
	}
}

// Do executes a function with retry logic
func Do(ctx context.Context, strategy *Strategy, fn func() error) error {
	if strategy == nil {
		strategy = DefaultStrategy()
	}

	var lastErr error
	delay := strategy.InitialDelay

	for attempt := 0; attempt < strategy.MaxAttempts; attempt++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		err := fn()
		if err == nil {
			return nil
		}

		lastErr = err

		// Check if error is retryable
		if !strategy.RetryableErrors(err) {
			return err
		}

		// Don't sleep after the last attempt
		if attempt < strategy.MaxAttempts-1 {
			calculatedDelay := calculateDelay(delay, strategy)

			// Wait with context cancellation support
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(calculatedDelay):
				delay = calculatedDelay
			}
		}
	}

	return fmt.Errorf("max attempts (%d) reached: %w", strategy.MaxAttempts, lastErr)
}

// calculateDelay calculates the next delay with exponential backoff and optional jitter
func calculateDelay(delay time.Duration, strategy *Strategy) time.Duration {
	calculatedDelay := min(time.Duration(float64(delay)*strategy.Multiplier), strategy.MaxDelay)

	if strategy.Jitter {
		jitter := time.Duration(rand.Float64() * float64(calculatedDelay) * 0.1)
		calculatedDelay = calculatedDelay + jitter
	}

	return calculatedDelay
}

// DoWithResult executes a function that returns a result with retry logic
func DoWithResult[T any](ctx context.Context, strategy *Strategy, fn func() (T, error)) (T, error) {
	var zero T
	if strategy == nil {
		strategy = DefaultStrategy()
	}

	var lastErr error
	delay := strategy.InitialDelay

	for attempt := 0; attempt < strategy.MaxAttempts; attempt++ {
		select {
		case <-ctx.Done():
			return zero, ctx.Err()
		default:
		}

		result, err := fn()
		if err == nil {
			return result, nil
		}

		lastErr = err

		if !strategy.RetryableErrors(err) {
			return zero, err
		}

		if attempt < strategy.MaxAttempts-1 {
			calculatedDelay := calculateDelay(delay, strategy)

			select {
			case <-ctx.Done():
				return zero, ctx.Err()
			case <-time.After(calculatedDelay):
				delay = calculatedDelay
			}
		}
	}

	return zero, fmt.Errorf("max attempts (%d) reached: %w", strategy.MaxAttempts, lastErr)
}

// ExponentialBackoff calculates the delay for exponential backoff.
// This is a utility function for implementing custom retry logic.
// The delay is calculated as: initialDelay * (multiplier ^ attempt), capped at maxDelay.
func ExponentialBackoff(attempt int, initialDelay time.Duration, maxDelay time.Duration, multiplier float64) time.Duration {
	delay := time.Duration(float64(initialDelay) * math.Pow(multiplier, float64(attempt)))
	return min(delay, maxDelay)
}

// IsRetryableError checks if an error should be retried.
// This is a utility function that can be used with Strategy.RetryableErrors.
// It returns false for context cancellation/timeout errors, true for all others.
// Common retryable errors: network errors, timeouts, 5xx status codes.
func IsRetryableError(err error) bool {
	if err == nil {
		return false
	}

	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}

	return true
}

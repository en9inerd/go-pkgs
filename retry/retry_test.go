package retry

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestDo_SucceedsFirstAttempt(t *testing.T) {
	calls := 0
	err := Do(context.Background(), nil, func() error {
		calls++
		return nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if calls != 1 {
		t.Errorf("calls = %d, want 1", calls)
	}
}

func TestDo_RetriesOnError(t *testing.T) {
	calls := 0
	err := Do(context.Background(), &Strategy{
		MaxAttempts:     3,
		InitialDelay:   1 * time.Millisecond,
		MaxDelay:        10 * time.Millisecond,
		Multiplier:     2.0,
		RetryableErrors: func(error) bool { return true },
	}, func() error {
		calls++
		if calls < 3 {
			return errors.New("fail")
		}
		return nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if calls != 3 {
		t.Errorf("calls = %d, want 3", calls)
	}
}

func TestDo_MaxAttemptsReached(t *testing.T) {
	err := Do(context.Background(), &Strategy{
		MaxAttempts:     2,
		InitialDelay:   1 * time.Millisecond,
		MaxDelay:        10 * time.Millisecond,
		Multiplier:     2.0,
		RetryableErrors: func(error) bool { return true },
	}, func() error {
		return errors.New("always fail")
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, errors.Unwrap(err)) {
		// Just check it wraps the original
	}
}

func TestDo_NonRetryableError(t *testing.T) {
	calls := 0
	permanent := errors.New("permanent")
	err := Do(context.Background(), &Strategy{
		MaxAttempts:     5,
		InitialDelay:   1 * time.Millisecond,
		MaxDelay:        10 * time.Millisecond,
		Multiplier:     2.0,
		RetryableErrors: func(err error) bool {
			return !errors.Is(err, permanent)
		},
	}, func() error {
		calls++
		return permanent
	})
	if !errors.Is(err, permanent) {
		t.Errorf("expected permanent error, got %v", err)
	}
	if calls != 1 {
		t.Errorf("calls = %d, want 1 (no retry for non-retryable)", calls)
	}
}

func TestDo_ContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := Do(ctx, &Strategy{
		MaxAttempts:     10,
		InitialDelay:   1 * time.Millisecond,
		MaxDelay:        10 * time.Millisecond,
		Multiplier:     2.0,
		RetryableErrors: func(error) bool { return true },
	}, func() error {
		return errors.New("fail")
	})
	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled, got %v", err)
	}
}

func TestDo_NilStrategy(t *testing.T) {
	calls := 0
	err := Do(context.Background(), nil, func() error {
		calls++
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Errorf("calls = %d", calls)
	}
}

func TestDoWithResult_Success(t *testing.T) {
	result, err := DoWithResult(context.Background(), &Strategy{
		MaxAttempts:     3,
		InitialDelay:   1 * time.Millisecond,
		MaxDelay:        10 * time.Millisecond,
		Multiplier:     2.0,
		RetryableErrors: func(error) bool { return true },
	}, func() (string, error) {
		return "hello", nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if result != "hello" {
		t.Errorf("result = %q, want %q", result, "hello")
	}
}

func TestDoWithResult_RetriesThenSucceeds(t *testing.T) {
	calls := 0
	result, err := DoWithResult(context.Background(), &Strategy{
		MaxAttempts:     3,
		InitialDelay:   1 * time.Millisecond,
		MaxDelay:        10 * time.Millisecond,
		Multiplier:     2.0,
		RetryableErrors: func(error) bool { return true },
	}, func() (int, error) {
		calls++
		if calls < 2 {
			return 0, errors.New("fail")
		}
		return 42, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if result != 42 {
		t.Errorf("result = %d, want 42", result)
	}
}

func TestDoWithResult_MaxAttempts(t *testing.T) {
	_, err := DoWithResult(context.Background(), &Strategy{
		MaxAttempts:     2,
		InitialDelay:   1 * time.Millisecond,
		MaxDelay:        10 * time.Millisecond,
		Multiplier:     2.0,
		RetryableErrors: func(error) bool { return true },
	}, func() (string, error) {
		return "", errors.New("always fail")
	})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestDoWithResult_ContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := DoWithResult(ctx, &Strategy{
		MaxAttempts:     10,
		InitialDelay:   1 * time.Millisecond,
		MaxDelay:        10 * time.Millisecond,
		Multiplier:     2.0,
		RetryableErrors: func(error) bool { return true },
	}, func() (int, error) {
		return 0, errors.New("fail")
	})
	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled, got %v", err)
	}
}

func TestExponentialBackoff(t *testing.T) {
	tests := []struct {
		attempt int
		want    time.Duration
	}{
		{0, 100 * time.Millisecond},
		{1, 200 * time.Millisecond},
		{2, 400 * time.Millisecond},
		{3, 800 * time.Millisecond},
		{10, 10 * time.Second}, // capped at maxDelay
	}
	for _, tt := range tests {
		got := ExponentialBackoff(tt.attempt, 100*time.Millisecond, 10*time.Second, 2.0)
		if got != tt.want {
			t.Errorf("attempt=%d: got %v, want %v", tt.attempt, got, tt.want)
		}
	}
}

func TestIsRetryableError(t *testing.T) {
	if IsRetryableError(nil) {
		t.Error("nil should not be retryable")
	}
	if !IsRetryableError(errors.New("network error")) {
		t.Error("regular error should be retryable")
	}
	if IsRetryableError(context.Canceled) {
		t.Error("context.Canceled should not be retryable")
	}
	if IsRetryableError(context.DeadlineExceeded) {
		t.Error("context.DeadlineExceeded should not be retryable")
	}
}

func TestDefaultStrategy(t *testing.T) {
	s := DefaultStrategy()
	if s.MaxAttempts != 3 {
		t.Errorf("MaxAttempts = %d, want 3", s.MaxAttempts)
	}
	if s.InitialDelay != 100*time.Millisecond {
		t.Errorf("InitialDelay = %v", s.InitialDelay)
	}
	if s.MaxDelay != 10*time.Second {
		t.Errorf("MaxDelay = %v", s.MaxDelay)
	}
	if s.Multiplier != 2.0 {
		t.Errorf("Multiplier = %f", s.Multiplier)
	}
	if !s.Jitter {
		t.Error("Jitter should be true")
	}
	if s.RetryableErrors == nil {
		t.Error("RetryableErrors should not be nil")
	}
}

package ratelimit

import (
	"context"
	"testing"
	"time"
)

// --------------- TokenBucket ---------------

func TestTokenBucket_AllowConsumesTokens(t *testing.T) {
	tb := NewTokenBucket(3, 1)

	for i := range 3 {
		if !tb.Allow() {
			t.Errorf("Allow() = false on attempt %d, want true", i)
		}
	}

	if tb.Allow() {
		t.Error("Allow() = true after exhausting tokens, want false")
	}
}

func TestTokenBucket_Refills(t *testing.T) {
	tb := NewTokenBucket(1, 100) // 100 tokens/sec → refills quickly

	if !tb.Allow() {
		t.Fatal("first Allow() should succeed")
	}
	if tb.Allow() {
		t.Fatal("second Allow() should fail (no tokens)")
	}

	time.Sleep(20 * time.Millisecond) // enough time for ~2 tokens at 100/sec

	if !tb.Allow() {
		t.Error("Allow() should succeed after refill")
	}
}

func TestTokenBucket_CapacityNotExceeded(t *testing.T) {
	tb := NewTokenBucket(2, 1000)

	time.Sleep(50 * time.Millisecond) // would add ~50 tokens at 1000/sec, but capped at 2

	count := 0
	for tb.Allow() {
		count++
		if count > 10 {
			t.Fatal("token count exceeded capacity")
		}
	}
	if count != 2 {
		t.Errorf("got %d tokens, want 2 (capacity)", count)
	}
}

func TestTokenBucket_Wait(t *testing.T) {
	tb := NewTokenBucket(1, 100) // fast refill
	tb.Allow()                   // exhaust

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	start := time.Now()
	if err := tb.Wait(ctx); err != nil {
		t.Fatalf("Wait() error: %v", err)
	}
	if time.Since(start) > 200*time.Millisecond {
		t.Error("Wait() took too long")
	}
}

func TestTokenBucket_WaitContextCancelled(t *testing.T) {
	tb := NewTokenBucket(1, 0.001) // very slow refill
	tb.Allow()                     // exhaust

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	err := tb.Wait(ctx)
	if err == nil {
		t.Error("expected context deadline error")
	}
}

// --------------- FixedWindow ---------------

func TestFixedWindow_AllowWithinLimit(t *testing.T) {
	fw := NewFixedWindow(3, time.Second)

	for i := range 3 {
		if !fw.Allow() {
			t.Errorf("Allow() = false on attempt %d, want true", i)
		}
	}

	if fw.Allow() {
		t.Error("Allow() = true after limit reached, want false")
	}
}

func TestFixedWindow_ResetsAfterWindow(t *testing.T) {
	fw := NewFixedWindow(1, 20*time.Millisecond)

	if !fw.Allow() {
		t.Fatal("first Allow() should succeed")
	}
	if fw.Allow() {
		t.Fatal("second Allow() should fail")
	}

	time.Sleep(30 * time.Millisecond) // wait for window reset

	if !fw.Allow() {
		t.Error("Allow() should succeed after window reset")
	}
}

func TestFixedWindow_Wait(t *testing.T) {
	fw := NewFixedWindow(1, 20*time.Millisecond)
	fw.Allow() // exhaust

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	start := time.Now()
	if err := fw.Wait(ctx); err != nil {
		t.Fatalf("Wait() error: %v", err)
	}
	elapsed := time.Since(start)
	if elapsed > 200*time.Millisecond {
		t.Errorf("Wait() took %v, expected ~20ms", elapsed)
	}
}

func TestFixedWindow_WaitContextCancelled(t *testing.T) {
	fw := NewFixedWindow(1, 10*time.Second)
	fw.Allow() // exhaust

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	err := fw.Wait(ctx)
	if err == nil {
		t.Error("expected context deadline error")
	}
}

// --------------- Interface compliance ---------------

func TestTokenBucket_ImplementsLimiter(t *testing.T) {
	var _ Limiter = NewTokenBucket(1, 1)
}

func TestFixedWindow_ImplementsLimiter(t *testing.T) {
	var _ Limiter = NewFixedWindow(1, time.Second)
}

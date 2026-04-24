package utils

import (
	"context"
	"testing"
	"time"
)

func TestRateLimiter_WaitWithContext(t *testing.T) {
	rl := NewRateLimiter(1000) // High limit — should not block.
	ctx := context.Background()
	if err := rl.Wait(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRateLimiter_WaitRespectsContextCancellation(t *testing.T) {
	rl := NewRateLimiter(1) // 1 RPS — second token won't be available immediately.

	// Consume the burst token.
	_ = rl.Wait(context.Background())

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	err := rl.Wait(ctx)
	if err == nil {
		t.Fatal("expected error from cancelled context, got nil")
	}
}

func TestRateLimiter_TryAcquire(t *testing.T) {
	rl := NewRateLimiter(100)
	if !rl.TryAcquire() {
		t.Fatal("expected TryAcquire to succeed on fresh limiter")
	}
}

func TestRateLimiter_SetRate(t *testing.T) {
	rl := NewRateLimiter(10)
	rl.SetRate(500) // Should not panic.
	if !rl.TryAcquire() {
		t.Fatal("expected TryAcquire to succeed after SetRate")
	}
}

func TestNewRateLimiter_ZeroRPSUsesDefault(t *testing.T) {
	rl := NewRateLimiter(0)
	if rl == nil {
		t.Fatal("expected non-nil RateLimiter")
	}
	if !rl.TryAcquire() {
		t.Fatal("expected TryAcquire to succeed with default high limit")
	}
}

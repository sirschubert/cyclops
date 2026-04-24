package utils

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestRetryWithBackoff_SucceedsFirstAttempt(t *testing.T) {
	calls := 0
	err := RetryWithBackoff(context.Background(), 3, func() error {
		calls++
		return nil
	})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if calls != 1 {
		t.Fatalf("expected 1 call, got %d", calls)
	}
}

func TestRetryWithBackoff_RetriesOnFailure(t *testing.T) {
	calls := 0
	want := errors.New("transient")
	err := RetryWithBackoff(context.Background(), 3, func() error {
		calls++
		if calls < 3 {
			return want
		}
		return nil
	})
	if err != nil {
		t.Fatalf("expected nil error after retries, got %v", err)
	}
	if calls != 3 {
		t.Fatalf("expected 3 calls, got %d", calls)
	}
}

func TestRetryWithBackoff_ReturnsLastError(t *testing.T) {
	sentinel := errors.New("permanent")
	err := RetryWithBackoff(context.Background(), 3, func() error {
		return sentinel
	})
	if !errors.Is(err, sentinel) {
		t.Fatalf("expected sentinel error, got %v", err)
	}
}

func TestRetryWithBackoff_RespectsContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	calls := 0
	// Cancel immediately so the second attempt never fires.
	go func() {
		time.Sleep(10 * time.Millisecond)
		cancel()
	}()

	err := RetryWithBackoff(ctx, 5, func() error {
		calls++
		return errors.New("fail")
	})

	if err == nil {
		t.Fatal("expected error due to cancellation")
	}
	if calls > 2 {
		t.Fatalf("expected ≤2 calls before cancellation, got %d", calls)
	}
}

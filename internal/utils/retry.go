package utils

import (
	"context"
	"time"
)

// RetryWithBackoff retries fn up to maxAttempts times with exponential backoff.
// It respects ctx cancellation between attempts. Returns the last error on
// exhaustion, or ctx.Err() if cancelled.
func RetryWithBackoff(ctx context.Context, maxAttempts int, fn func() error) error {
	delay := 500 * time.Millisecond
	var lastErr error
	for attempt := 0; attempt < maxAttempts; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(delay):
			}
			delay *= 2
			if delay > 30*time.Second {
				delay = 30 * time.Second
			}
		}
		if lastErr = fn(); lastErr == nil {
			return nil
		}
	}
	return lastErr
}

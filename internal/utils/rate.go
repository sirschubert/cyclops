package utils

import (
	"context"
	"time"

	"golang.org/x/time/rate"
)

// RateLimiter wraps a token bucket rate limiter with convenience methods
type RateLimiter struct {
	limiter *rate.Limiter
}

// NewRateLimiter creates a rate limiter with the given requests per second
func NewRateLimiter(rps int) *RateLimiter {
	if rps <= 0 {
		rps = 1000 // default high limit
	}
	return &RateLimiter{
		limiter: rate.NewLimiter(rate.Limit(rps), rps),
	}
}

// Wait blocks until a token is available, respecting ctx cancellation.
func (rl *RateLimiter) Wait(ctx context.Context) error {
	return rl.limiter.Wait(ctx)
}

// TryAcquire attempts to acquire a token without blocking
func (rl *RateLimiter) TryAcquire() bool {
	return rl.limiter.Allow()
}

// SetRate updates the rate limit dynamically
func (rl *RateLimiter) SetRate(rps int) {
	if rps > 0 {
		rl.limiter.SetLimit(rate.Limit(rps))
		rl.limiter.SetBurst(rps)
	}
}

// Throttle sleeps for the given duration to spread requests
func Throttle(duration time.Duration) {
	time.Sleep(duration)
}

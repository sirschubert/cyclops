package utils

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
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

// Autotuner dynamically adjusts a RateLimiter based on observed response codes.
//
// Algorithm:
//   - Every 30 seconds with no 429/503 responses: rate += 50 req/s (up to ceiling)
//   - On any 429 or 503: immediately rate /= 2 (down to floor)
//
// Floor: 10 req/s. Ceiling: 2000 req/s.
type Autotuner struct {
	mu      sync.Mutex
	cur     int
	limiter *RateLimiter
	floor   int
	ceil    int
	verbose bool
	hits    int // 429/503 count in the current 30-second window
	done    chan struct{}
}

// NewAutotuner creates an Autotuner starting at startRate req/s.
func NewAutotuner(startRate int, limiter *RateLimiter, verbose bool) *Autotuner {
	if startRate < 10 {
		startRate = 10
	}
	if startRate > 2000 {
		startRate = 2000
	}
	return &Autotuner{
		cur:     startRate,
		limiter: limiter,
		floor:   10,
		ceil:    2000,
		verbose: verbose,
		done:    make(chan struct{}),
	}
}

// ReportCode should be called with every HTTP response status code.
// On 429 or 503 it immediately halves the rate.
func (a *Autotuner) ReportCode(code int) {
	if code != 429 && code != 503 {
		return
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	prev := a.cur
	a.cur = a.cur / 2
	if a.cur < a.floor {
		a.cur = a.floor
	}
	if a.cur != prev {
		a.limiter.SetRate(a.cur)
		if a.verbose {
			slog.Debug(fmt.Sprintf("[autotune] rate adjusted: %d → %d req/s", prev, a.cur))
		}
	}
	a.hits++
}

// Run starts the periodic 30-second adjustment loop. Call as a goroutine.
// It exits when ctx is cancelled or Stop() is called.
func (a *Autotuner) Run(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-a.done:
			return
		case <-ticker.C:
			a.mu.Lock()
			if a.hits == 0 {
				prev := a.cur
				a.cur += 50
				if a.cur > a.ceil {
					a.cur = a.ceil
				}
				if a.cur != prev {
					a.limiter.SetRate(a.cur)
					if a.verbose {
						slog.Debug(fmt.Sprintf("[autotune] rate adjusted: %d → %d req/s", prev, a.cur))
					}
				}
			}
			a.hits = 0
			a.mu.Unlock()
		}
	}
}

// Stop terminates the Run loop.
func (a *Autotuner) Stop() {
	select {
	case <-a.done:
	default:
		close(a.done)
	}
}

package utils

import (
	"context"
	"sync"

	"golang.org/x/time/rate"
)

// WorkerPool manages a pool of workers that process jobs concurrently
type WorkerPool struct {
	workers   int
	jobs      chan func()
	wg        sync.WaitGroup
	ctx       context.Context
	cancel    context.CancelFunc
	limiter   *rate.Limiter
}

// NewWorkerPool creates a new worker pool with the given number of workers and rate limit
func NewWorkerPool(workers int, rps int) *WorkerPool {
	ctx, cancel := context.WithCancel(context.Background())

	var limiter *rate.Limiter
	if rps > 0 {
		limiter = rate.NewLimiter(rate.Limit(rps), rps)
	}

	return &WorkerPool{
		workers: workers,
		jobs:    make(chan func(), workers*2),
		ctx:     ctx,
		cancel:  cancel,
		limiter: limiter,
	}
}

// Start begins the worker goroutines
func (wp *WorkerPool) Start() {
	for i := 0; i < wp.workers; i++ {
		wp.wg.Add(1)
		go wp.worker()
	}
}

// worker is the goroutine that processes jobs
func (wp *WorkerPool) worker() {
	defer wp.wg.Done()
	for {
		select {
		case <-wp.ctx.Done():
			return
		case job, ok := <-wp.jobs:
			if !ok {
				return
			}
			if wp.limiter != nil {
				_ = wp.limiter.Wait(wp.ctx)
			}
			job()
		}
	}
}

// Submit adds a job to the pool
func (wp *WorkerPool) Submit(job func()) {
	select {
	case wp.jobs <- job:
	case <-wp.ctx.Done():
		return
	}
}

// Stop gracefully shuts down the worker pool
func (wp *WorkerPool) Stop() {
	wp.cancel()
	close(wp.jobs)
	wp.wg.Wait()
}

// Wait blocks until all submitted jobs are processed
func (wp *WorkerPool) Wait() {
	wp.wg.Wait()
}

package parallel

import (
	"context"
	"sync"
)

// Collect runs fn for each index [0, n) with bounded concurrency,
// collecting results into a slice that preserves index order.
// A concurrency value <= 0 is treated as "auto" (= n, i.e. unlimited).
// ctx is passed to each fn invocation so callers can enforce per-item deadlines.
// On cancellation, goroutines waiting for a semaphore slot exit early; their
// result entries hold the zero value of T.
// A panic in fn is recovered on its worker and re-raised on the caller's
// goroutine once all workers finish (first panic wins; the worker stack is lost).
func Collect[T any](ctx context.Context, n, concurrency int, fn func(ctx context.Context, i int) T) []T {
	if n == 0 {
		return nil
	}
	if concurrency <= 0 {
		concurrency = n
	}

	results := make([]T, n)
	var wg sync.WaitGroup
	sem := make(chan struct{}, concurrency)
	var panics Panics

	for i := range n {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			defer panics.Recover()
			select {
			case sem <- struct{}{}:
			case <-ctx.Done():
				return
			}
			defer func() { <-sem }()

			results[idx] = fn(ctx, idx)
		}(i)
	}
	wg.Wait()
	panics.Repanic()
	return results
}

// Panics carries the first panic from a set of worker goroutines back to the
// goroutine that waits on them, where a caller's recover (e.g. cmd.Execute's
// observability finalizer) can see it. The worker's stack trace is lost.
//
// Per worker: `defer wg.Done()` first, then `defer panics.Recover()`. After
// wg.Wait(): `panics.Repanic()`.
type Panics struct {
	once sync.Once
	val  any
	set  bool
}

// Recover must be deferred directly (`defer p.Recover()`) for recover() to work.
func (p *Panics) Recover() {
	if v := recover(); v != nil {
		p.once.Do(func() { p.val, p.set = v, true })
	}
}

// Repanic re-raises the first recovered panic, if any. Call it after wg.Wait().
func (p *Panics) Repanic() {
	if p.set {
		panic(p.val)
	}
}

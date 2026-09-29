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
	var firstPanic panicSlot

	for i := range n {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			defer func() { firstPanic.record(recover()) }()
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
	firstPanic.repanic()
	return results
}

// panicSlot holds the first panic value recovered from any worker.
type panicSlot struct {
	once sync.Once
	val  any
	set  bool
}

func (s *panicSlot) record(p any) {
	if p == nil {
		return
	}
	s.once.Do(func() { s.val, s.set = p, true })
}

func (s *panicSlot) repanic() {
	if s.set {
		panic(s.val)
	}
}

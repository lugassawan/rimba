package parallel

import (
	"context"
	"sync"
)

// Collect runs fn for each index [0, n) with bounded concurrency,
// collecting results into a slice that preserves index order.
// A concurrency value <= 0 is treated as "auto" (= n, i.e. unlimited).
// ctx is passed to each fn invocation so callers can enforce per-item deadlines.
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

	for i := range n {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
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
	return results
}

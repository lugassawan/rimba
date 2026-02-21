package parallel

import "sync"

// Collect runs fn for each index [0, n) with bounded concurrency,
// collecting results into a slice that preserves index order.
func Collect[T any](n, concurrency int, fn func(i int) T) []T {
	if n == 0 {
		return nil
	}

	results := make([]T, n)
	var wg sync.WaitGroup
	sem := make(chan struct{}, concurrency)

	for i := range n {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			results[idx] = fn(idx)
		}(i)
	}
	wg.Wait()
	return results
}

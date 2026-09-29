package parallel

import (
	"context"
	"fmt"
	"io"
	"os"
	"runtime/debug"
	"sync"
)

// Collect runs fn for each index [0, n) with bounded concurrency,
// collecting results into a slice that preserves index order.
// A concurrency value <= 0 is treated as "auto" (= n, i.e. unlimited).
// ctx is passed to each fn invocation so callers can enforce per-item deadlines.
// On cancellation, goroutines waiting for a semaphore slot exit early; their
// result entries hold the zero value of T.
// A worker panic is re-raised on the caller's goroutine once all workers
// finish (see Group).
func Collect[T any](ctx context.Context, n, concurrency int, fn func(ctx context.Context, i int) T) []T {
	if n == 0 {
		return nil
	}
	if concurrency <= 0 {
		concurrency = n
	}

	results := make([]T, n)
	var g Group
	sem := make(chan struct{}, concurrency)

	for i := range n {
		g.Go(func() {
			select {
			case sem <- struct{}{}:
			case <-ctx.Done():
				return
			}
			defer func() { <-sem }()

			results[i] = fn(ctx, i)
		})
	}
	g.Wait()
	return results
}

// Group is a sync.WaitGroup whose workers' panics reach the goroutine that
// calls Wait, where a caller's recover (e.g. cmd.Execute's observability
// finalizer) can see them. First panic wins; its worker stack goes to stderr.
type Group struct {
	wg sync.WaitGroup
	p  panicSlot
}

// Go runs fn on a new goroutine, recovering any panic for Wait to re-raise.
func (g *Group) Go(fn func()) {
	g.wg.Go(func() {
		defer g.p.recoverWorker()
		fn()
	})
}

// Wait blocks until every Go worker returns, then re-raises the first worker panic.
func (g *Group) Wait() {
	g.wg.Wait()
	if g.p.set {
		ReportPanic("panic in worker goroutine", g.p.val, g.p.stack)
		panic(g.p.val)
	}
}

// ReportPanic writes a recovered panic and the stack it was captured with to
// stderr under label, for panics that are re-raised or swallowed elsewhere.
func ReportPanic(label string, p any, stack []byte) { writePanic(os.Stderr, label, p, stack) }

// panicSlot holds the first panic recovered from any worker.
type panicSlot struct {
	once  sync.Once
	val   any
	stack []byte
	set   bool
}

// recoverWorker must be deferred directly for recover() to take effect.
func (s *panicSlot) recoverWorker() {
	if v := recover(); v != nil {
		stack := debug.Stack() // still inside the deferred call: includes the panicking frames
		s.once.Do(func() { s.val, s.stack, s.set = v, stack, true })
	}
}

func writePanic(w io.Writer, label string, p any, stack []byte) {
	_, _ = fmt.Fprintf(w, "%s: %v\n%s\n", label, p, stack)
}

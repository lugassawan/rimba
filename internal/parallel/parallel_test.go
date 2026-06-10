package parallel_test

import (
	"context"
	"sync/atomic"
	"testing"

	"github.com/lugassawan/rimba/internal/parallel"
)

func TestCollectPreservesOrder(t *testing.T) {
	results := parallel.Collect(context.Background(), 10, 4, func(_ context.Context, i int) int {
		return i * 2
	})

	if len(results) != 10 {
		t.Fatalf("got %d results, want 10", len(results))
	}
	for i, v := range results {
		if v != i*2 {
			t.Errorf("results[%d] = %d, want %d", i, v, i*2)
		}
	}
}

func TestCollectZeroItems(t *testing.T) {
	results := parallel.Collect(context.Background(), 0, 8, func(_ context.Context, i int) string {
		t.Fatal("fn should not be called for n=0")
		return ""
	})

	if results != nil {
		t.Errorf("expected nil for n=0, got %v", results)
	}
}

func TestCollectAutoConcurrency(t *testing.T) {
	// concurrency=0 must not deadlock; treat as n (auto).
	results := parallel.Collect(context.Background(), 5, 0, func(_ context.Context, i int) int { return i })
	if len(results) != 5 {
		t.Fatalf("got %d results, want 5", len(results))
	}
	for i, v := range results {
		if v != i {
			t.Errorf("results[%d] = %d, want %d", i, v, i)
		}
	}

	// negative concurrency also treated as auto.
	results2 := parallel.Collect(context.Background(), 3, -1, func(_ context.Context, i int) int { return i * 10 })
	if len(results2) != 3 {
		t.Fatalf("got %d results, want 3", len(results2))
	}
	for i, v := range results2 {
		if v != i*10 {
			t.Errorf("results2[%d] = %d, want %d", i, v, i*10)
		}
	}
}

func TestCollectBoundsConcurrency(t *testing.T) {
	const maxConcurrency = 2
	var running atomic.Int32
	var maxSeen atomic.Int32

	parallel.Collect(context.Background(), 20, maxConcurrency, func(_ context.Context, i int) struct{} {
		cur := running.Add(1)
		for {
			old := maxSeen.Load()
			if cur <= old || maxSeen.CompareAndSwap(old, cur) {
				break
			}
		}
		running.Add(-1)
		return struct{}{}
	})

	if maxSeen.Load() > maxConcurrency {
		t.Errorf("max concurrent = %d, want <= %d", maxSeen.Load(), maxConcurrency)
	}
}

type testCtxKey struct{}

func TestCollectPassesCtx(t *testing.T) {
	ctx := context.WithValue(context.Background(), testCtxKey{}, "marker")

	parallel.Collect(ctx, 3, 3, func(itemCtx context.Context, _ int) struct{} {
		if v, ok := itemCtx.Value(testCtxKey{}).(string); !ok || v != "marker" {
			t.Error("ctx not propagated to fn")
		}
		return struct{}{}
	})
}

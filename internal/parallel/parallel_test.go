package parallel_test

import (
	"sync/atomic"
	"testing"

	"github.com/lugassawan/rimba/internal/parallel"
)

func TestCollectPreservesOrder(t *testing.T) {
	results := parallel.Collect(10, 4, func(i int) int {
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
	results := parallel.Collect(0, 8, func(i int) string {
		t.Fatal("fn should not be called for n=0")
		return ""
	})

	if results != nil {
		t.Errorf("expected nil for n=0, got %v", results)
	}
}

func TestCollectBoundsConcurrency(t *testing.T) {
	const maxConcurrency = 2
	var running atomic.Int32
	var maxSeen atomic.Int32

	parallel.Collect(20, maxConcurrency, func(i int) struct{} {
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

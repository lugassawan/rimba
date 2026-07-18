package deps

import (
	"context"
	"testing"
)

// benchHooks simulates a repo with several independent post-create hooks —
// short shell commands with a small sleep to stand in for real work (linters,
// codegen, etc.) without making the benchmark itself slow to run.
func benchHooks() []string {
	return []string{
		"sleep 0.01",
		"sleep 0.01",
		"sleep 0.01",
		"sleep 0.01",
	}
}

// BenchmarkRunPostCreateHooksSerial measures the pre-existing (and still
// default) serial execution path.
func BenchmarkRunPostCreateHooksSerial(b *testing.B) {
	dir := b.TempDir()
	hooks := benchHooks()

	b.ResetTimer()
	for b.Loop() {
		RunPostCreateHooks(context.Background(), dir, hooks, false, nil)
	}
}

// BenchmarkRunPostCreateHooksParallel measures the opt-in [hooks] parallel =
// true path added by this change. Run both with `go test -bench=RunPostCreateHooks
// -benchmem ./internal/deps/...` to compare before/after wall-clock —
// parallel should approach the single-slowest-hook duration instead of the
// sum of all hooks, proportional to hook count (a single-hook config sees no
// speedup, by design — nothing to parallelize).
func BenchmarkRunPostCreateHooksParallel(b *testing.B) {
	dir := b.TempDir()
	hooks := benchHooks()

	b.ResetTimer()
	for b.Loop() {
		RunPostCreateHooks(context.Background(), dir, hooks, true, nil)
	}
}

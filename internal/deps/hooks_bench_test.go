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

// serialStages splits hooks into one single-command stage each, the
// canonical form a flat, non-parallel post_create list normalizes to.
func serialStages(hooks []string) [][]string {
	stages := make([][]string, len(hooks))
	for i, h := range hooks {
		stages[i] = []string{h}
	}
	return stages
}

// BenchmarkRunPostCreateHooksSerial measures the pre-existing (and still
// default) serial execution path.
func BenchmarkRunPostCreateHooksSerial(b *testing.B) {
	dir := b.TempDir()
	stages := serialStages(benchHooks())

	b.ResetTimer()
	for b.Loop() {
		RunPostCreateHooks(context.Background(), dir, stages, nil)
	}
}

// BenchmarkRunPostCreateHooksParallel measures the multi-command-stage
// (concurrent) execution path. Run both with `go test -bench=RunPostCreateHooks
// -benchmem ./internal/deps/...` to compare before/after wall-clock —
// the multi-command stage should approach the single-slowest-hook duration
// instead of the sum of all hooks, proportional to hook count (a single-hook
// stage sees no speedup, by design — nothing to parallelize).
func BenchmarkRunPostCreateHooksParallel(b *testing.B) {
	dir := b.TempDir()
	stages := [][]string{benchHooks()}

	b.ResetTimer()
	for b.Loop() {
		RunPostCreateHooks(context.Background(), dir, stages, nil)
	}
}

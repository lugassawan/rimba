package cmd

import (
	"context"
	"sync"
	"testing"

	"github.com/lugassawan/rimba/internal/git"
)

// mergeBenchMockRunner simulates IsDirty calls.
type mergeBenchMockRunner struct{}

func (m *mergeBenchMockRunner) Run(_ context.Context, _ ...string) (string, error) {
	return "", nil
}
func (m *mergeBenchMockRunner) RunInDir(_ context.Context, _ string, _ ...string) (string, error) {
	return "", nil
}

func BenchmarkDirtyChecksSequential(b *testing.B) {
	r := &mergeBenchMockRunner{}
	srcPath := "/tmp/wt-src"
	tgtPath := "/tmp/wt-tgt"

	b.ResetTimer()
	for b.Loop() {
		_, _ = git.IsDirty(context.Background(), r, srcPath)
		_, _ = git.IsDirty(context.Background(), r, tgtPath)
	}
}

func BenchmarkDirtyChecksParallel(b *testing.B) {
	r := &mergeBenchMockRunner{}
	srcPath := "/tmp/wt-src"
	tgtPath := "/tmp/wt-tgt"

	b.ResetTimer()
	for b.Loop() {
		var srcResult, tgtResult struct {
			dirty bool
			err   error
		}
		var wg sync.WaitGroup
		wg.Add(2)
		go func() {
			defer wg.Done()
			srcResult.dirty, srcResult.err = git.IsDirty(context.Background(), r, srcPath)
		}()
		go func() {
			defer wg.Done()
			tgtResult.dirty, tgtResult.err = git.IsDirty(context.Background(), r, tgtPath)
		}()
		wg.Wait()
		_ = srcResult
		_ = tgtResult
	}
}

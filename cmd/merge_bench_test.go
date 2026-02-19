package cmd

import (
	"sync"
	"testing"

	"github.com/lugassawan/rimba/internal/git"
)

// mergeBenchMockRunner simulates IsDirty calls.
type mergeBenchMockRunner struct{}

func (m *mergeBenchMockRunner) Run(_ ...string) (string, error)                { return "", nil }
func (m *mergeBenchMockRunner) RunInDir(_ string, _ ...string) (string, error) { return "", nil }

func BenchmarkDirtyChecksSequential(b *testing.B) {
	r := &mergeBenchMockRunner{}
	srcPath := "/tmp/wt-src"
	tgtPath := "/tmp/wt-tgt"

	b.ResetTimer()
	for b.Loop() {
		_, _ = git.IsDirty(r, srcPath)
		_, _ = git.IsDirty(r, tgtPath)
	}
}

func BenchmarkDirtyChecksParallel(b *testing.B) {
	r := &mergeBenchMockRunner{}
	srcPath := "/tmp/wt-src"
	tgtPath := "/tmp/wt-tgt"

	b.ResetTimer()
	for b.Loop() {
		var srcResult, tgtResult dirtyResult
		var wg sync.WaitGroup
		wg.Add(2)
		go func() {
			defer wg.Done()
			srcResult.dirty, srcResult.err = git.IsDirty(r, srcPath)
		}()
		go func() {
			defer wg.Done()
			tgtResult.dirty, tgtResult.err = git.IsDirty(r, tgtPath)
		}()
		wg.Wait()
		_ = srcResult
		_ = tgtResult
	}
}

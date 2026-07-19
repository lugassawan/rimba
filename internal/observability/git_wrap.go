package observability

import (
	"context"
	"time"

	"github.com/lugassawan/rimba/internal/git"
)

// recordingRunner decorates a git.Runner so every subprocess call is logged
// via a Recorder.
type recordingRunner struct {
	inner git.Runner
	rec   *Recorder
}

// WrapRunner decorates r so every git subprocess is recorded via rec. Returns r
// unchanged when rec is nil (observability disabled) — callers can always wrap
// unconditionally.
func WrapRunner(r git.Runner, rec *Recorder) git.Runner {
	if rec == nil {
		return r
	}
	return &recordingRunner{inner: r, rec: rec}
}

// Run executes args via the inner runner and records the invocation.
func (w *recordingRunner) Run(ctx context.Context, args ...string) (string, error) {
	start := time.Now()
	out, err := w.inner.Run(ctx, args...)
	w.log("", args, start, err)
	return out, err
}

// RunInDir executes args in dir via the inner runner and records the invocation.
func (w *recordingRunner) RunInDir(ctx context.Context, dir string, args ...string) (string, error) {
	start := time.Now()
	out, err := w.inner.RunInDir(ctx, dir, args...)
	w.log(dir, args, start, err)
	return out, err
}

// log records one git subprocess invocation to w.rec.
func (w *recordingRunner) log(dir string, args []string, start time.Time, err error) {
	// git.Runner doesn't expose a real process exit code (see Global Constraints);
	// 0/-1 is an accepted approximation, distinguishable from a real git exit code
	// of 0 only in that -1 can never occur for git itself.
	exitCode := 0
	if err != nil {
		exitCode = -1
	}
	w.rec.LogSubprocess(CategoryGit, dir, args, exitCode, time.Since(start), errString(err), err != nil)
}

package observability

import (
	"context"
	"time"

	"github.com/lugassawan/rimba/internal/debug"
	"github.com/lugassawan/rimba/internal/git"
)

// recordingRunner decorates a git.Runner, recording each subprocess call via
// whichever Recorder is attached to that call's ctx — see WrapRunner.
type recordingRunner struct {
	inner git.Runner
}

// WrapRunner decorates r so every git subprocess is recorded via whichever
// Recorder is on each call's ctx — looked up fresh per call (not captured at
// wrap time), so MCP's one long-lived Runner instance still records each
// tool call correctly. Falls back to RIMBA_DEBUG's stderr timer when absent.
func WrapRunner(r git.Runner) git.Runner {
	return &recordingRunner{inner: r}
}

// Run executes args via the inner runner and records the invocation.
func (w *recordingRunner) Run(ctx context.Context, args ...string) (string, error) {
	start := time.Now()
	out, err := w.inner.Run(ctx, args...)
	w.log(ctx, "", args, start, err)
	return out, err
}

// RunInDir executes args in dir via the inner runner and records the invocation.
func (w *recordingRunner) RunInDir(ctx context.Context, dir string, args ...string) (string, error) {
	start := time.Now()
	out, err := w.inner.RunInDir(ctx, dir, args...)
	w.log(ctx, dir, args, start, err)
	return out, err
}

// log records one git subprocess invocation to whatever Recorder is attached
// to ctx, or falls back to the RIMBA_DEBUG stderr timer when ctx carries none.
func (w *recordingRunner) log(ctx context.Context, dir string, args []string, start time.Time, err error) {
	rec := FromContext(ctx)
	if rec == nil {
		debug.LogGitTiming(dir, args, time.Since(start))
		return
	}
	// git.Runner exposes no real exit code; 0/-1 approximates it (-1 can
	// never be a genuine git exit code, so it's distinguishable from success).
	exitCode := 0
	if err != nil {
		exitCode = -1
	}
	rec.LogSubprocess(CategoryGit, dir, args, exitCode, time.Since(start), errString(err), err != nil)
}

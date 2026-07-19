package gh

import (
	"context"
	"time"

	"github.com/lugassawan/rimba/internal/observability"
)

// recordingRunner decorates a Runner, recording each gh subprocess call via
// whichever Recorder is attached to that call's ctx — see WrapRunner.
type recordingRunner struct {
	inner Runner
}

// WrapRunner decorates r so every gh subprocess is recorded via whichever
// Recorder is on each call's ctx — the same per-call design as
// observability.WrapRunner, for the same MCP reason. gh has no
// RIMBA_DEBUG fallback; it was never covered by that timer.
func WrapRunner(r Runner) Runner {
	return &recordingRunner{inner: r}
}

// Run executes args via the inner runner and records the invocation.
func (w *recordingRunner) Run(ctx context.Context, args ...string) ([]byte, error) {
	start := time.Now()
	out, err := w.inner.Run(ctx, args...)

	rec := observability.FromContext(ctx)
	if rec == nil {
		return out, err
	}
	// gh's Runner doesn't expose a real process exit code; 0/-1 is the same
	// accepted approximation used for git.Runner (see internal/observability's
	// git decorator).
	exitCode := 0
	if err != nil {
		exitCode = -1
	}
	errStr := ""
	if err != nil {
		errStr = err.Error()
	}
	rec.LogSubprocess(observability.CategoryGH, "", args, exitCode, time.Since(start), errStr, err != nil)
	return out, err
}

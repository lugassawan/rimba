package gh

import (
	"context"
	"time"

	"github.com/lugassawan/rimba/internal/observability"
)

// recordingRunner decorates a Runner so every gh subprocess call is logged
// via whichever Recorder is attached to that specific call's ctx (looked up
// fresh on each call, never captured once at wrap time). This lets a single
// long-lived wrapped Runner — as the MCP server's HandlerContext holds for
// its whole process lifetime — correctly record each tool call's own
// per-call Recorder.
type recordingRunner struct {
	inner Runner
}

// WrapRunner decorates r so every gh subprocess is recorded via whichever
// Recorder is attached to each call's ctx. Safe to call unconditionally — a
// call whose ctx carries no Recorder simply isn't recorded (gh has no
// RIMBA_DEBUG fallback timer, unlike git).
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

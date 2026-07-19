package observability

import (
	"context"
	"time"

	"github.com/lugassawan/rimba/internal/debug"
	"github.com/lugassawan/rimba/internal/git"
)

// recordingRunner decorates a git.Runner so every subprocess call is logged
// via whichever Recorder is attached to that specific call's ctx (looked up
// fresh on each call, never captured once at wrap time). This lets a single
// long-lived wrapped Runner — as the MCP server's HandlerContext holds for
// its whole process lifetime — correctly record each tool call's own
// per-call Recorder, and correctly no-op for a call whose ctx carries none.
type recordingRunner struct {
	inner git.Runner
}

// WrapRunner decorates r so every git subprocess is recorded via whichever
// Recorder is attached to each call's ctx. Safe to call unconditionally —
// a call whose ctx carries no Recorder falls back to the RIMBA_DEBUG stderr
// timer (via debug.LogGitTiming) so --debug/RIMBA_DEBUG keeps working even
// when observability is disabled for that invocation.
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
	// git.Runner doesn't expose a real process exit code (see Global Constraints);
	// 0/-1 is an accepted approximation, distinguishable from a real git exit code
	// of 0 only in that -1 can never occur for git itself.
	exitCode := 0
	if err != nil {
		exitCode = -1
	}
	rec.LogSubprocess(CategoryGit, dir, args, exitCode, time.Since(start), errString(err), err != nil)
}

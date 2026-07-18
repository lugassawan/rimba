package deps

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"sync/atomic"
	"time"

	"github.com/lugassawan/rimba/internal/observability"
	"github.com/lugassawan/rimba/internal/parallel"
	"github.com/lugassawan/rimba/internal/progress"
)

// HookResult holds the outcome of a post-create hook execution.
type HookResult struct {
	Command string
	Error   error
}

// RunPostCreateHooks executes shell commands in the worktree directory.
// Skips launching new hooks when ctx is already cancelled; kills any in-flight
// hook subprocess when ctx is cancelled (via exec.CommandContext).
//
// When parallel is false (the default), hooks run serially in input order:
// launching stops (without starting the next hook) once ctx is cancelled, and
// a failing hook does not stop later hooks from running. This mode is
// byte-for-byte unchanged from before parallel execution existed.
//
// When parallel is true, all hooks are launched concurrently via
// parallel.Collect; a failing or cancelled hook still does not stop the
// others. The returned slice keeps input order regardless of completion
// order, and every entry's Command always names the hook it corresponds to
// — see runHooksParallel's doc comment for how a hook that never got to run
// because ctx was already cancelled is reported (as a failure, not a silent
// success).
func RunPostCreateHooks(ctx context.Context, worktreeDir string, hooks []string, parallel bool, onProgress progress.Func) []HookResult {
	rec := observability.FromContext(ctx)
	if parallel {
		return runHooksParallel(ctx, worktreeDir, hooks, rec, onProgress)
	}
	return runHooksSerial(ctx, worktreeDir, hooks, rec, onProgress)
}

// runHooksSerial runs hooks one at a time, stopping before launching the next
// hook once ctx is cancelled. This is the original RunPostCreateHooks
// behavior, unchanged.
func runHooksSerial(ctx context.Context, worktreeDir string, hooks []string, rec *observability.Recorder, onProgress progress.Func) []HookResult {
	results := make([]HookResult, 0, len(hooks))
	for i, hook := range hooks {
		if ctx.Err() != nil {
			break
		}
		progress.Notifyf(onProgress, "%s (%d/%d)", hook, i+1, len(hooks))
		results = append(results, runHook(ctx, worktreeDir, hook, rec))
	}
	return results
}

// runHooksParallel runs all hooks concurrently via parallel.Collect, mirroring
// internal/deps/manager.go's install method: an atomic.Int32 completion
// counter drives "%d/%d complete" progress notifications instead of the
// serial mode's per-hook ordinal message, since ordinals don't mean the same
// thing once hooks run concurrently.
//
// Cancellation-timing caveat (observed empirically, see
// internal/deps/hooks_ctx_test.go): unlike the serial loop's explicit
// `if ctx.Err() != nil { break }` check before each hook, parallel.Collect
// races each hook's dispatch against ctx.Done() internally via a select —
// even with concurrency == len(hooks) (no semaphore contention), Go's
// pseudo-random select between two simultaneously-ready channels means a
// hook can lose that race and never call runHook at all. Left alone, its
// slot in the returned slice would keep the zero HookResult{} (empty
// Command, nil Error) — indistinguishable from "a hook with an empty
// command succeeded," which is misleading since no such hook exists. The
// fixup loop below turns any such slot into {Command: <the real hook>,
// Error: ctx.Err()} so an un-launched hook is always reported as failed
// with its real command, never as a silent phantom success.
func runHooksParallel(ctx context.Context, worktreeDir string, hooks []string, rec *observability.Recorder, onProgress progress.Func) []HookResult {
	total := len(hooks)
	var done atomic.Int32
	// Concurrency 0 (unbounded) is intentional: unlike deps modules, hooks
	// are user-configured and typically few, so there's no auto-detected
	// count to guard against — no need for a DepsConcurrency()-style cap.
	results := parallel.Collect(ctx, total, 0, func(ctx context.Context, i int) HookResult {
		res := runHook(ctx, worktreeDir, hooks[i], rec)
		completed := done.Add(1)
		progress.Notifyf(onProgress, "%d/%d complete", completed, total)
		return res
	})

	if ctx.Err() != nil {
		for i, r := range results {
			// Accepted residual edge case: a hook with a genuinely empty-string
			// Command that legitimately succeeds is indistinguishable from an
			// un-launched slot and would be mislabeled as failed here.
			if r.Command == "" && r.Error == nil {
				results[i] = HookResult{Command: hooks[i], Error: ctx.Err()}
			}
		}
	}
	return results
}

// runHook executes a single hook command and records its outcome. Shared by
// both serial and parallel execution paths.
func runHook(ctx context.Context, worktreeDir, hook string, rec *observability.Recorder) HookResult {
	cmd := exec.CommandContext(ctx, "sh", "-c", hook) //nolint:gosec // hook commands come from user config
	cmd.Dir = worktreeDir
	configureProcessGroup(cmd)

	var buf tailBuffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf

	start := time.Now()
	err := cmd.Run()
	exitCode := 0
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			exitCode = exitErr.ExitCode()
		}
	}
	rec.LogSubprocess(observability.CategoryHook, worktreeDir, []string{hook}, exitCode, time.Since(start), buf.String(), err != nil)

	if err != nil {
		err = fmt.Errorf("hook %q: %w\n%s", hook, err, strings.TrimSpace(buf.String()))
	}
	return HookResult{Command: hook, Error: err}
}

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

// RunPostCreateHooks executes shell commands in the worktree directory,
// organized into stages: stages run in order, and every command within a
// stage runs concurrently. A flat hook list normalizes to one single-command
// stage per hook (serial) or one stage holding every hook (parallel) — see
// internal/config's PostCreateStages/PostRenameStages/NormalizeHookStages for
// how the [hooks] parallel flag and the nested-array config shape both
// collapse into this canonical stages form before reaching here.
//
// Cancellation is handled per stage type, matching each one's pre-existing
// behavior exactly rather than gating at the stage-loop level: a
// single-command stage stops before launching once ctx is cancelled (so a
// pre-cancelled ctx yields 0 results for that stage — serial's original
// behavior); a multi-command stage still dispatches to parallel.Collect
// regardless of ctx's state at entry, always yielding one result per command
// (parallel's original "never silently drops a slot" invariant) — see
// runStageParallel's doc comment. A failing hook never stops others from
// running, whether they're in the same stage or a later one.
//
// This means a single cancelled run of a staged config can report
// asymmetrically: a cancelled single-command stage's hooks are silently
// absent from the returned slice, while a cancelled multi-command stage's
// hooks are all present, each marked failed with ctx.Err(). Callers
// displaying HookResults should not assume its length always equals the
// input hook count when ctx may be cancelled mid-run.
//
// Progress messages preserve each of the two historical formats depending on
// stage shape: a single-command stage reports "<hook> (i/N)" (the original
// serial per-hook ordinal, byte-for-byte unchanged for the common
// all-single-command-stages case), a multi-command stage reports
// "N/total complete" (the original parallel completion counter). Both share
// one continuous atomic counter spanning the whole run, not reset per stage,
// so numbering stays correct across a mix of stage shapes.
func RunPostCreateHooks(ctx context.Context, worktreeDir string, stages [][]string, onProgress progress.Func) []HookResult {
	rec := observability.FromContext(ctx)

	total := 0
	for _, stage := range stages {
		total += len(stage)
	}

	var results []HookResult
	var done atomic.Int32
	for _, stage := range stages {
		if len(stage) <= 1 {
			results = append(results, runStageSerial(ctx, worktreeDir, stage, rec, &done, total, onProgress)...)
		} else {
			results = append(results, runStageParallel(ctx, worktreeDir, stage, rec, &done, total, onProgress)...)
		}
	}
	return results
}

// runStageSerial runs a stage's commands one at a time (used for 0- or
// 1-command stages), stopping before launching the next once ctx is
// cancelled — this is the original RunPostCreateHooks serial behavior,
// unchanged. done is the run-wide atomic.Int32 progress counter, shared with
// runStageParallel, so numbering stays continuous across stage boundaries
// regardless of stage shape.
func runStageSerial(ctx context.Context, worktreeDir string, hooks []string, rec *observability.Recorder, done *atomic.Int32, total int, onProgress progress.Func) []HookResult {
	results := make([]HookResult, 0, len(hooks))
	for _, hook := range hooks {
		if ctx.Err() != nil {
			break
		}
		ordinal := done.Add(1)
		progress.Notifyf(onProgress, "%s (%d/%d)", hook, ordinal, total)
		results = append(results, runHook(ctx, worktreeDir, hook, rec))
	}
	return results
}

// runStageParallel runs a stage's commands concurrently via parallel.Collect,
// mirroring internal/deps/manager.go's install method. done is the run-wide
// atomic.Int32 progress counter, shared with runStageSerial, driving
// continuous "%d/%d complete" progress notifications.
//
// Cancellation-timing caveat (observed empirically, see
// internal/deps/hooks_ctx_test.go): unlike runStageSerial's explicit
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
func runStageParallel(ctx context.Context, worktreeDir string, hooks []string, rec *observability.Recorder, done *atomic.Int32, total int, onProgress progress.Func) []HookResult {
	n := len(hooks)
	// Concurrency 0 (unbounded) is intentional: unlike deps modules, hooks
	// are user-configured and typically few, so there's no auto-detected
	// count to guard against — no need for a DepsConcurrency()-style cap.
	results := parallel.Collect(ctx, n, 0, func(ctx context.Context, i int) HookResult {
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
		exitCode = -1
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

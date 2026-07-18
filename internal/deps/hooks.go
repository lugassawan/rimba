package deps

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/lugassawan/rimba/internal/observability"
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
func RunPostCreateHooks(ctx context.Context, worktreeDir string, hooks []string, onProgress progress.Func) []HookResult {
	rec := observability.FromContext(ctx)
	results := make([]HookResult, 0, len(hooks))
	for i, hook := range hooks {
		if ctx.Err() != nil {
			break
		}
		progress.Notifyf(onProgress, "%s (%d/%d)", hook, i+1, len(hooks))

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
		results = append(results, HookResult{Command: hook, Error: err})
	}
	return results
}

package deps

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/lugassawan/rimba/internal/progress"
)

// HookResult holds the outcome of a post-create hook execution.
type HookResult struct {
	Command string
	Error   error
}

// RunPostCreateHooks executes shell commands in the worktree directory.
// It stops launching new hooks if ctx is cancelled, but does not stop an already-running hook.
func RunPostCreateHooks(ctx context.Context, worktreeDir string, hooks []string, onProgress progress.Func) []HookResult {
	results := make([]HookResult, 0, len(hooks))
	for i, hook := range hooks {
		if ctx.Err() != nil {
			break
		}
		progress.Notifyf(onProgress, "%s (%d/%d)", hook, i+1, len(hooks))

		cmd := exec.CommandContext(ctx, "sh", "-c", hook) //nolint:gosec // hook commands come from user config
		cmd.Dir = worktreeDir

		var buf bytes.Buffer
		cmd.Stdout = &buf
		cmd.Stderr = &buf

		err := cmd.Run()
		if err != nil {
			err = fmt.Errorf("hook %q: %w\n%s", hook, err, strings.TrimSpace(buf.String()))
		}
		results = append(results, HookResult{Command: hook, Error: err})
	}
	return results
}

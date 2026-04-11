package deps

import (
	"bytes"
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
// It collects errors but does not stop on failure — all hooks run regardless.
func RunPostCreateHooks(worktreeDir string, hooks []string, onProgress progress.Func) []HookResult {
	results := make([]HookResult, 0, len(hooks))
	for i, hook := range hooks {
		progress.Notifyf(onProgress, "%s (%d/%d)", hook, i+1, len(hooks))

		cmd := exec.Command("sh", "-c", hook) //nolint:gosec // hook commands come from user config
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

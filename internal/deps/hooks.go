package deps

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
)

// HookResult holds the outcome of a post-create hook execution.
type HookResult struct {
	Command string
	Error   error
}

// RunPostCreateHooks executes shell commands in the worktree directory.
// It collects errors but does not stop on failure — all hooks run regardless.
func RunPostCreateHooks(worktreeDir string, hooks []string, onProgress ProgressFunc) []HookResult {
	results := make([]HookResult, 0, len(hooks))
	for i, hook := range hooks {
		if onProgress != nil {
			onProgress(i+1, len(hooks), hook)
		}

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

package deps

import (
	"fmt"
	"os"
	"os/exec"
)

// HookResult holds the outcome of a post-create hook execution.
type HookResult struct {
	Command string
	Error   error
}

// RunPostCreateHooks executes shell commands in the worktree directory.
// It collects errors but does not stop on failure — all hooks run regardless.
func RunPostCreateHooks(worktreeDir string, hooks []string) []HookResult {
	results := make([]HookResult, 0, len(hooks))
	for _, hook := range hooks {
		cmd := exec.Command("sh", "-c", hook)
		cmd.Dir = worktreeDir
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		err := cmd.Run()
		if err != nil {
			err = fmt.Errorf("hook %q: %w", hook, err)
		}
		results = append(results, HookResult{Command: hook, Error: err})
	}
	return results
}

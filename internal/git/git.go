package git

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"
)

// gracefulShutdownDelay bounds how long a cancelled subprocess gets to react
// to SIGTERM before the backstop SIGKILL fires.
const gracefulShutdownDelay = 5 * time.Second

// Runner abstracts git command execution for testability.
type Runner interface {
	Run(ctx context.Context, args ...string) (string, error)
	RunInDir(ctx context.Context, dir string, args ...string) (string, error)
}

// ExecRunner is the production implementation of Runner.
type ExecRunner struct {
	// Dir is the working directory for git commands. If empty, uses the current directory.
	Dir string
	// Timeout, when positive, applies a per-invocation deadline to every subprocess.
	// Zero means no timeout (relies solely on the caller's context).
	Timeout time.Duration
}

// isLongRunning reports whether args is a worktree add/remove/move operation,
// which on large trees can exceed a finite per-command deadline (#380).
func isLongRunning(args []string) bool {
	return len(args) >= 2 && args[0] == cmdWorktree &&
		(args[1] == "add" || args[1] == "remove" || args[1] == "move")
}

// RunInDir is the single execution primitive: all other methods delegate here.
func (r *ExecRunner) RunInDir(ctx context.Context, dir string, args ...string) (string, error) {
	if r.Timeout > 0 && !isLongRunning(args) {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, r.Timeout)
		defer cancel()
	}

	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Env = stableGitEnv(os.Environ())
	if dir != "" {
		cmd.Dir = dir
	}
	// Send SIGTERM (not the default SIGKILL) on cancellation so git's own
	// atexit handler can unlink its index.lock; WaitDelay is the backstop if
	// git ignores the signal. See #380.
	cmd.Cancel = func() error { return cmd.Process.Signal(syscall.SIGTERM) }
	cmd.WaitDelay = gracefulShutdownDelay

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	result := strings.TrimSpace(stdout.String())
	if err != nil {
		return result, fmt.Errorf("git %s: %s: %w", strings.Join(args, " "), errMsg(strings.TrimSpace(stderr.String()), result), err)
	}
	return result, nil
}

// errMsg builds the error text from stderr and stdout: both when both are
// present, otherwise whichever one is non-empty.
func errMsg(stderr, stdout string) string {
	switch {
	case stderr != "" && stdout != "":
		return stderr + "\n" + stdout
	case stderr != "":
		return stderr
	default:
		return stdout
	}
}

func stableGitEnv(environ []string) []string {
	env := make([]string, 0, len(environ)+2)
	for _, entry := range environ {
		key, _, _ := strings.Cut(entry, "=")
		if strings.EqualFold(key, "LANG") || strings.EqualFold(key, "LC_ALL") {
			continue
		}
		env = append(env, entry)
	}

	return append(env, "LANG=C", "LC_ALL=C")
}

func (r *ExecRunner) Run(ctx context.Context, args ...string) (string, error) {
	return r.RunInDir(ctx, r.Dir, args...)
}

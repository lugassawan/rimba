package git

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

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

// RunInDir is the single execution primitive: all other methods delegate here.
func (r *ExecRunner) RunInDir(ctx context.Context, dir string, args ...string) (string, error) {
	if r.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, r.Timeout)
		defer cancel()
	}

	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Env = stableGitEnv(os.Environ())
	if dir != "" {
		cmd.Dir = dir
	}

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
// present (so a caller matching on either stream never silently loses the
// other), otherwise whichever one is non-empty.
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

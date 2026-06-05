package git

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// Runner abstracts git command execution for testability.
type Runner interface {
	Run(args ...string) (string, error)
	RunInDir(dir string, args ...string) (string, error)
	RunContext(ctx context.Context, args ...string) (string, error)
	RunInDirContext(ctx context.Context, dir string, args ...string) (string, error)
}

// ExecRunner is the production implementation of Runner.
type ExecRunner struct {
	// Dir is the working directory for git commands. If empty, uses the current directory.
	Dir string
}

// RunInDirContext is the single execution primitive: all other methods delegate here.
func (r *ExecRunner) RunInDirContext(ctx context.Context, dir string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Env = stableGitEnv(os.Environ())
	if dir != "" {
		cmd.Dir = dir
	}

	out, err := cmd.CombinedOutput()
	result := strings.TrimSpace(string(out))
	if err != nil {
		return result, fmt.Errorf("git %s: %s: %w", strings.Join(args, " "), result, err)
	}
	return result, nil
}

func stableGitEnv(environ []string) []string {
	env := make([]string, 0, len(environ)+2)
	for _, entry := range environ {
		key, _, ok := strings.Cut(entry, "=")
		if !ok {
			env = append(env, entry)
			continue
		}
		if strings.EqualFold(key, "LANG") || strings.EqualFold(key, "LC_ALL") {
			continue
		}
		env = append(env, entry)
	}

	return append(env, "LANG=C", "LC_ALL=C")
}

func (r *ExecRunner) RunContext(ctx context.Context, args ...string) (string, error) {
	return r.RunInDirContext(ctx, r.Dir, args...)
}

func (r *ExecRunner) Run(args ...string) (string, error) {
	return r.RunInDirContext(context.Background(), r.Dir, args...)
}

func (r *ExecRunner) RunInDir(dir string, args ...string) (string, error) {
	return r.RunInDirContext(context.Background(), dir, args...)
}

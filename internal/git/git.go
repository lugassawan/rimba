package git

import (
	"fmt"
	"os/exec"
	"strings"
)

// Runner abstracts git command execution for testability.
type Runner interface {
	Run(args ...string) (string, error)
	RunInDir(dir string, args ...string) (string, error)
}

// ExecRunner is the production implementation of Runner.
type ExecRunner struct {
	// Dir is the working directory for git commands. If empty, uses the current directory.
	Dir string
}

func (r *ExecRunner) Run(args ...string) (string, error) {
	return r.RunInDir(r.Dir, args...)
}

func (r *ExecRunner) RunInDir(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
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

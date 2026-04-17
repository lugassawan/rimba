package gh

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// Runner abstracts `gh` command execution so callers can inject fakes in tests.
type Runner interface {
	Run(ctx context.Context, args ...string) ([]byte, error)
}

// Default returns a Runner that shells out to the real `gh` binary.
func Default() Runner {
	return &execRunner{}
}

type execRunner struct{}

func (r *execRunner) Run(ctx context.Context, args ...string) ([]byte, error) {
	out, err := exec.CommandContext(ctx, "gh", args...).CombinedOutput()
	if err != nil {
		return out, fmt.Errorf("gh %s: %s: %w", strings.Join(args, " "), strings.TrimSpace(string(out)), err)
	}
	return out, nil
}

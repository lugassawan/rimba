package gh

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// Runner runs `gh` commands. Tests inject a fake.
type Runner interface {
	Run(ctx context.Context, args ...string) ([]byte, error)
}

// Default returns a Runner backed by the real `gh` binary.
// When timeout is positive, each invocation is bounded by that deadline.
func Default(timeout time.Duration) Runner {
	return &execRunner{timeout: timeout}
}

type execRunner struct {
	timeout time.Duration
}

func (r *execRunner) Run(ctx context.Context, args ...string) ([]byte, error) {
	if r.timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, r.timeout)
		defer cancel()
	}
	out, err := exec.CommandContext(ctx, "gh", args...).CombinedOutput()
	if err != nil {
		return out, fmt.Errorf("gh %s: %s: %w", strings.Join(args, " "), strings.TrimSpace(string(out)), err)
	}
	return out, nil
}

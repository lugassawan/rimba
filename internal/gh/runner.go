package gh

import (
	"bytes"
	"context"
	"fmt"
	"os"
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
	cmd := exec.CommandContext(ctx, "gh", args...)
	// gh's update-notifier can write to stderr on a zero-exit run; keep it off stdout.
	cmd.Env = append(os.Environ(), "GH_NO_UPDATE_NOTIFIER=1")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		msg := errMsg(strings.TrimSpace(stderr.String()), strings.TrimSpace(string(out)))
		return out, fmt.Errorf("gh %s: %s: %w", strings.Join(args, " "), msg, err)
	}
	return out, nil
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

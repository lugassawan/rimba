package executor

import (
	"context"
	"errors"
	"os/exec"
	"sync"
)

// RunFunc executes a shell command in a directory and returns its output.
// Non-zero exit codes are reported via exitCode (not err).
// err is non-nil only when the process could not start.
type RunFunc func(ctx context.Context, dir, command string) (stdout, stderr []byte, exitCode int, err error)

// Target represents a worktree to execute a command in.
type Target struct {
	Path   string
	Branch string
	Task   string
}

// Config bundles all parameters for a parallel execution run.
type Config struct {
	Targets     []Target
	Command     string
	Concurrency int // 0 = len(Targets)
	FailFast    bool
	Runner      RunFunc
}

// Result holds the outcome of executing a command in a single target.
type Result struct {
	Target    Target
	ExitCode  int
	Stdout    []byte
	Stderr    []byte
	Err       error // non-nil only if process couldn't start
	Cancelled bool  // true if skipped due to fail-fast
}

// Run executes cfg.Command in each target directory concurrently.
// Results are returned in the same order as cfg.Targets.
func Run(ctx context.Context, cfg Config) []Result {
	if len(cfg.Targets) == 0 {
		return nil
	}

	concurrency := cfg.Concurrency
	if concurrency <= 0 {
		concurrency = len(cfg.Targets)
	}

	results := make([]Result, len(cfg.Targets))
	sem := make(chan struct{}, concurrency)

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	var wg sync.WaitGroup

	for i, t := range cfg.Targets {
		wg.Add(1)
		go func(idx int, target Target) {
			defer wg.Done()

			// Check for cancellation before acquiring semaphore.
			select {
			case <-ctx.Done():
				results[idx] = Result{Target: target, Cancelled: true}
				return
			case sem <- struct{}{}:
			}
			defer func() { <-sem }()

			// Re-check after acquiring semaphore.
			select {
			case <-ctx.Done():
				results[idx] = Result{Target: target, Cancelled: true}
				return
			default:
			}

			stdout, stderr, exitCode, err := cfg.Runner(ctx, target.Path, cfg.Command)
			results[idx] = Result{
				Target:   target,
				ExitCode: exitCode,
				Stdout:   stdout,
				Stderr:   stderr,
				Err:      err,
			}

			if cfg.FailFast && (exitCode != 0 || err != nil) {
				cancel()
			}
		}(i, t)
	}

	wg.Wait()
	return results
}

// ShellRunner returns a RunFunc that executes commands via "sh -c".
func ShellRunner() RunFunc {
	return func(ctx context.Context, dir, command string) ([]byte, []byte, int, error) {
		cmd := exec.CommandContext(ctx, "sh", "-c", command)
		cmd.Dir = dir

		var stdout, stderr safeBuffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr

		err := cmd.Run()
		if err != nil {
			var exitErr *exec.ExitError
			if errors.As(err, &exitErr) {
				return stdout.Bytes(), stderr.Bytes(), exitErr.ExitCode(), nil
			}
			// Process could not start (e.g. binary not found).
			return nil, nil, 0, err
		}

		return stdout.Bytes(), stderr.Bytes(), 0, nil
	}
}

// safeBuffer is a minimal concurrency-safe bytes.Buffer for capturing output.
type safeBuffer struct {
	mu  sync.Mutex
	buf []byte
}

func (b *safeBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	b.buf = append(b.buf, p...)
	b.mu.Unlock()
	return len(p), nil
}

func (b *safeBuffer) Bytes() []byte {
	b.mu.Lock()
	defer b.mu.Unlock()
	return append([]byte(nil), b.buf...)
}

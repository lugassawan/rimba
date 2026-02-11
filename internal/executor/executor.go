package executor

import (
	"bytes"
	"context"
	"os/exec"
	"sync"
	"time"
)

// Config holds configuration for parallel command execution.
type Config struct {
	Concurrency int
	FailFast    bool
	Targets     []Target
	Command     []string
}

// Target represents a single worktree to execute a command in.
type Target struct {
	Task   string
	Branch string
	Path   string
}

// Result holds the outcome of executing a command in a single worktree.
type Result struct {
	Target   Target
	ExitCode int
	Stdout   string
	Stderr   string
	Duration time.Duration
	Err      error
}

// Run executes cfg.Command in each target worktree with bounded concurrency.
// If FailFast is set, remaining targets are cancelled on the first non-zero exit.
func Run(ctx context.Context, cfg Config) []Result {
	if cfg.Concurrency < 1 {
		cfg.Concurrency = 4
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	results := make([]Result, len(cfg.Targets))
	var mu sync.Mutex
	var wg sync.WaitGroup
	sem := make(chan struct{}, cfg.Concurrency)

	var failed bool

	for i, target := range cfg.Targets {
		wg.Add(1)
		go func(idx int, t Target) {
			defer wg.Done()

			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				results[idx] = Result{
					Target: t,
					Err:    ctx.Err(),
				}
				return
			}

			r := execute(ctx, t, cfg.Command)
			results[idx] = r

			if cfg.FailFast && r.ExitCode != 0 && r.Err == nil {
				mu.Lock()
				if !failed {
					failed = true
					cancel()
				}
				mu.Unlock()
			}
		}(i, target)
	}
	wg.Wait()

	return results
}

func execute(ctx context.Context, t Target, command []string) Result {
	start := time.Now()

	cmd := exec.CommandContext(ctx, command[0], command[1:]...) //nolint:gosec // user-specified command is intentional
	cmd.Dir = t.Path

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	r := Result{
		Target:   t,
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
		Duration: time.Since(start),
	}

	if err != nil {
		var exitErr *exec.ExitError
		if ok := isExitError(err, &exitErr); ok {
			r.ExitCode = exitErr.ExitCode()
		} else {
			r.Err = err
		}
	}

	return r
}

func isExitError(err error, target **exec.ExitError) bool {
	if e, ok := err.(*exec.ExitError); ok { //nolint:errorlint // exec.Run returns *ExitError directly
		*target = e
		return true
	}
	return false
}

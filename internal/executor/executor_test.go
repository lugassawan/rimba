package executor

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"testing"
	"time"
)

func mockRunner(stdout, stderr string, exitCode int, err error) RunFunc {
	return func(_ context.Context, _, _ string) ([]byte, []byte, int, error) {
		return []byte(stdout), []byte(stderr), exitCode, err
	}
}

func TestRunSuccess(t *testing.T) {
	results := Run(context.Background(), Config{
		Targets: []Target{{Path: "/tmp", Branch: "feature/x", Task: "x"}},
		Command: "echo hello",
		Runner:  mockRunner("hello\n", "", 0, nil),
	})

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	r := results[0]
	if r.ExitCode != 0 {
		t.Errorf("expected exit 0, got %d", r.ExitCode)
	}
	if string(r.Stdout) != "hello\n" {
		t.Errorf("expected stdout %q, got %q", "hello\n", string(r.Stdout))
	}
	if r.Err != nil {
		t.Errorf("expected no error, got %v", r.Err)
	}
	if r.Cancelled {
		t.Error("expected not cancelled")
	}
}

func TestRunMultiple(t *testing.T) {
	targets := []Target{
		{Path: "/a", Task: "a"},
		{Path: "/b", Task: "b"},
		{Path: "/c", Task: "c"},
	}

	callIdx := atomic.Int32{}
	runner := func(_ context.Context, dir, _ string) ([]byte, []byte, int, error) {
		callIdx.Add(1)
		return []byte("out:" + dir), nil, 0, nil
	}

	results := Run(context.Background(), Config{
		Targets: targets,
		Command: "cmd",
		Runner:  runner,
	})

	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}
	// Verify ordering is preserved regardless of execution order.
	for i, r := range results {
		expected := "out:" + targets[i].Path
		if string(r.Stdout) != expected {
			t.Errorf("result[%d]: expected stdout %q, got %q", i, expected, string(r.Stdout))
		}
	}
}

func TestRunNonZeroExit(t *testing.T) {
	results := Run(context.Background(), Config{
		Targets: []Target{{Path: "/tmp", Task: "x"}},
		Command: "false",
		Runner:  mockRunner("", "error output", 1, nil),
	})

	r := results[0]
	if r.ExitCode != 1 {
		t.Errorf("expected exit 1, got %d", r.ExitCode)
	}
	if r.Err != nil {
		t.Errorf("expected nil error, got %v", r.Err)
	}
	if string(r.Stderr) != "error output" {
		t.Errorf("expected stderr %q, got %q", "error output", string(r.Stderr))
	}
}

func TestRunProcessError(t *testing.T) {
	procErr := errors.New("exec: not found")
	results := Run(context.Background(), Config{
		Targets: []Target{{Path: "/tmp", Task: "x"}},
		Command: "nonexistent",
		Runner:  mockRunner("", "", 0, procErr),
	})

	r := results[0]
	if r.Err == nil {
		t.Fatal("expected error, got nil")
	}
	if r.Err.Error() != "exec: not found" {
		t.Errorf("expected error %q, got %q", "exec: not found", r.Err.Error())
	}
}

func TestRunFailFastCancels(t *testing.T) {
	// Use a gate channel to ensure /b and /c goroutines are blocked
	// until /a has failed and the context is cancelled.
	gate := make(chan struct{})
	results := Run(context.Background(), Config{
		Targets: []Target{
			{Path: "/a", Task: "a"},
			{Path: "/b", Task: "b"},
			{Path: "/c", Task: "c"},
		},
		Command:  "cmd",
		FailFast: true,
		Runner: func(ctx context.Context, dir, _ string) ([]byte, []byte, int, error) {
			if dir == "/a" {
				return nil, nil, 1, nil // first target fails → cancels context
			}
			// Wait for context cancellation or gate (never opened).
			select {
			case <-ctx.Done():
				return nil, nil, 0, ctx.Err()
			case <-gate:
				return nil, nil, 0, nil
			}
		},
	})
	close(gate) // cleanup

	if results[0].ExitCode != 1 {
		t.Errorf("first result: expected exit 1, got %d", results[0].ExitCode)
	}

	// The remaining targets should either be cancelled or have a context error.
	for i, r := range results[1:] {
		if !r.Cancelled && r.Err == nil {
			t.Errorf("result[%d]: expected cancelled or error, got ExitCode=%d Err=%v Cancelled=%v",
				i+1, r.ExitCode, r.Err, r.Cancelled)
		}
	}
}

func TestRunConcurrencyLimit(t *testing.T) {
	var maxConcurrent atomic.Int32
	var current atomic.Int32

	runner := func(_ context.Context, _, _ string) ([]byte, []byte, int, error) {
		c := current.Add(1)
		for {
			old := maxConcurrent.Load()
			if c <= old || maxConcurrent.CompareAndSwap(old, c) {
				break
			}
		}
		time.Sleep(10 * time.Millisecond) // hold the slot briefly
		current.Add(-1)
		return nil, nil, 0, nil
	}

	targets := make([]Target, 10)
	for i := range targets {
		targets[i] = Target{Path: fmt.Sprintf("/t%d", i), Task: fmt.Sprintf("t%d", i)}
	}

	Run(context.Background(), Config{
		Targets:     targets,
		Command:     "cmd",
		Concurrency: 2,
		Runner:      runner,
	})

	if m := maxConcurrent.Load(); m > 2 {
		t.Errorf("expected max concurrency 2, got %d", m)
	}
}

func TestRunEmptyTargets(t *testing.T) {
	results := Run(context.Background(), Config{
		Targets: nil,
		Command: "echo hi",
		Runner:  mockRunner("", "", 0, nil),
	})

	if results != nil {
		t.Errorf("expected nil results, got %d", len(results))
	}
}

func TestRunContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	results := Run(ctx, Config{
		Targets: []Target{
			{Path: "/a", Task: "a"},
			{Path: "/b", Task: "b"},
		},
		Command: "cmd",
		Runner: func(_ context.Context, _, _ string) ([]byte, []byte, int, error) {
			return nil, nil, 0, nil
		},
	})

	cancelled := 0
	for _, r := range results {
		if r.Cancelled {
			cancelled++
		}
	}
	if cancelled == 0 {
		t.Error("expected at least one cancelled result")
	}
}

func TestShellRunnerSuccess(t *testing.T) {
	runner := ShellRunner()
	dir := t.TempDir()

	stdout, stderr, exitCode, err := runner(context.Background(), dir, "echo hello")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if exitCode != 0 {
		t.Errorf("expected exit 0, got %d", exitCode)
	}
	if got := string(stdout); got != "hello\n" {
		t.Errorf("stdout = %q, want %q", got, "hello\n")
	}
	if len(stderr) != 0 {
		t.Errorf("expected empty stderr, got %q", string(stderr))
	}
}

func TestShellRunnerNonZeroExit(t *testing.T) {
	runner := ShellRunner()
	dir := t.TempDir()

	stdout, stderr, exitCode, err := runner(context.Background(), dir, "echo fail >&2; exit 42")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if exitCode != 42 {
		t.Errorf("expected exit 42, got %d", exitCode)
	}
	if len(stdout) != 0 {
		t.Errorf("expected empty stdout, got %q", string(stdout))
	}
	if got := string(stderr); got != "fail\n" {
		t.Errorf("stderr = %q, want %q", got, "fail\n")
	}
}

func TestShellRunnerProcessError(t *testing.T) {
	runner := ShellRunner()

	// Use a non-existent directory to cause a start error.
	_, _, _, err := runner(context.Background(), "/nonexistent-dir-abc123", "echo hi")
	if err == nil {
		t.Fatal("expected error for non-existent directory")
	}
}

func TestRunStdoutStderrSeparate(t *testing.T) {
	results := Run(context.Background(), Config{
		Targets: []Target{{Path: "/tmp", Task: "x"}},
		Command: "cmd",
		Runner:  mockRunner("out data", "err data", 0, nil),
	})

	r := results[0]
	if string(r.Stdout) != "out data" {
		t.Errorf("expected stdout %q, got %q", "out data", string(r.Stdout))
	}
	if string(r.Stderr) != "err data" {
		t.Errorf("expected stderr %q, got %q", "err data", string(r.Stderr))
	}
}

package executor

import (
	"context"
	"runtime"
	"testing"
)

const (
	testBranchA = "feature/a"
	testBranchB = "feature/b"
)

func TestRunEchoAll(t *testing.T) {
	targets := []Target{
		{Task: "a", Branch: testBranchA, Path: t.TempDir()},
		{Task: "b", Branch: testBranchB, Path: t.TempDir()},
	}

	cfg := Config{
		Concurrency: 2,
		Targets:     targets,
		Command:     echoCommand("hello"),
	}

	results := Run(context.Background(), cfg)

	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	for i, r := range results {
		if r.ExitCode != 0 {
			t.Errorf("result[%d]: expected exit 0, got %d (err=%v, stderr=%s)", i, r.ExitCode, r.Err, r.Stderr)
		}
		if r.Err != nil {
			t.Errorf("result[%d]: unexpected error: %v", i, r.Err)
		}
		if r.Duration == 0 {
			t.Errorf("result[%d]: expected non-zero duration", i)
		}
	}
}

func TestRunFailFastCancels(t *testing.T) {
	targets := make([]Target, 10)
	for i := range targets {
		targets[i] = Target{Task: "t", Branch: "feature/t", Path: t.TempDir()}
	}

	cfg := Config{
		Concurrency: 1, // serial so we can observe cancellation
		FailFast:    true,
		Targets:     targets,
		Command:     failCommand(),
	}

	results := Run(context.Background(), cfg)

	var nonZero, cancelled int
	for _, r := range results {
		if r.ExitCode != 0 {
			nonZero++
		}
		if r.Err != nil {
			cancelled++
		}
	}

	if nonZero == 0 {
		t.Error("expected at least one non-zero exit code")
	}
	if cancelled == 0 {
		t.Error("expected at least one cancelled result with fail-fast")
	}
}

func TestRunDefaultConcurrency(t *testing.T) {
	cfg := Config{
		Targets: []Target{{Task: "a", Branch: testBranchA, Path: t.TempDir()}},
		Command: echoCommand("ok"),
	}

	results := Run(context.Background(), cfg)

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].ExitCode != 0 {
		t.Errorf("expected exit 0, got %d", results[0].ExitCode)
	}
}

func TestRunContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	cfg := Config{
		Concurrency: 1,
		Targets:     []Target{{Task: "a", Branch: testBranchA, Path: t.TempDir()}},
		Command:     echoCommand("hello"),
	}

	results := Run(ctx, cfg)

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Err == nil {
		t.Error("expected error from cancelled context")
	}
}

func TestRunEmptyTargets(t *testing.T) {
	cfg := Config{
		Command: echoCommand("hello"),
	}
	results := Run(context.Background(), cfg)
	if len(results) != 0 {
		t.Fatalf("expected 0 results, got %d", len(results))
	}
}

// echoCommand returns a platform-appropriate echo command.
func echoCommand(msg string) []string {
	if runtime.GOOS == "windows" {
		return []string{"cmd", "/c", "echo " + msg}
	}
	return []string{"echo", msg}
}

// failCommand returns a platform-appropriate command that exits with code 1.
func failCommand() []string {
	if runtime.GOOS == "windows" {
		return []string{"cmd", "/c", "exit 1"}
	}
	return []string{"false"}
}

package deps

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestRunPostCreateHooksCancelledStopsLaunching(t *testing.T) {
	dir := t.TempDir()
	marker := filepath.Join(dir, "marker")

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // pre-cancel so no hooks launch

	start := time.Now()
	results := RunPostCreateHooks(ctx, dir, []string{"sleep 5", "touch " + marker}, false, nil)
	elapsed := time.Since(start)

	if elapsed > time.Second {
		t.Errorf("expected fast return on pre-cancelled ctx, took %v", elapsed)
	}
	if _, err := os.Stat(marker); err == nil {
		t.Error("marker file should not have been created (second hook never ran)")
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results for pre-cancelled ctx, got %d", len(results))
	}
}

func TestRunPostCreateHooksKillsChild(t *testing.T) {
	dir := t.TempDir()

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan []HookResult, 1)
	go func() {
		done <- RunPostCreateHooks(ctx, dir, []string{"sleep 30"}, false, nil)
	}()

	time.Sleep(50 * time.Millisecond)
	cancel()

	select {
	case results := <-done:
		if len(results) == 0 || results[0].Error == nil {
			t.Error("expected a non-nil error for killed hook")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("RunPostCreateHooks did not return within 2s after cancellation")
	}
}

// TestRunPostCreateHooksParallelPreCancelledNoPhantomSuccess documents (and
// guards against regressing) the parallel-mode cancellation behavior found
// while implementing this task: parallel.Collect races each hook's dispatch
// against ctx.Done() via a select, and — even with concurrency==len(hooks),
// so no semaphore contention — Go's pseudo-random choice between two
// simultaneously-ready channels means some hooks lose that race and never
// call runHook. Without the fixup in runHooksParallel, those slots would
// keep the Go zero value HookResult{Command: "", Error: nil}, which reads as
// a phantom hook that silently "succeeded". This test asserts every result
// names its real hook and carries a non-nil error instead.
func TestRunPostCreateHooksParallelPreCancelledNoPhantomSuccess(t *testing.T) {
	dir := t.TempDir()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // pre-cancel so every hook races an already-closed ctx.Done()

	hooks := make([]string, 20) // wide enough that the select races both ways
	for i := range hooks {
		hooks[i] = "touch marker-" + string(rune('a'+i)) + ".txt"
	}

	results := RunPostCreateHooks(ctx, dir, hooks, true, nil)
	if len(results) != len(hooks) {
		t.Fatalf("expected %d results (parallel.Collect always returns len==n), got %d", len(hooks), len(results))
	}
	for i, r := range results {
		if r.Command != hooks[i] {
			t.Errorf("results[%d].Command = %q, want %q (must never be empty/phantom)", i, r.Command, hooks[i])
		}
		if r.Error == nil {
			t.Errorf("results[%d].Error = nil, want non-nil — a hook that never ran must not look like a success", i)
		}
	}
}

// TestRunPostCreateHooksParallelCancelMidFlightKillsChildren documents the
// mid-flight case: hooks that already acquired a slot and started running
// get killed via exec.CommandContext, same as serial mode's per-hook kill —
// this part of parallel mode's cancellation behavior matches serial's.
func TestRunPostCreateHooksParallelCancelMidFlightKillsChildren(t *testing.T) {
	dir := t.TempDir()

	ctx, cancel := context.WithCancel(context.Background())

	hooks := make([]string, 10)
	for i := range hooks {
		hooks[i] = "sleep 30"
	}

	done := make(chan []HookResult, 1)
	go func() {
		done <- RunPostCreateHooks(ctx, dir, hooks, true, nil)
	}()

	time.Sleep(50 * time.Millisecond)
	cancel()

	select {
	case results := <-done:
		if len(results) != len(hooks) {
			t.Fatalf("expected %d results, got %d", len(hooks), len(results))
		}
		for i, r := range results {
			if r.Command != hooks[i] {
				t.Errorf("results[%d].Command = %q, want %q", i, r.Command, hooks[i])
			}
			if r.Error == nil {
				t.Errorf("results[%d]: expected a non-nil error for a cancelled/killed hook", i)
			}
		}
	case <-time.After(2 * time.Second):
		t.Fatal("RunPostCreateHooks did not return within 2s after cancellation")
	}
}

func TestRunInstallCancelledKillsChild(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	mod := Module{
		Dir:        "node_modules",
		InstallCmd: "sleep 30",
	}

	done := make(chan error, 1)
	go func() {
		done <- runInstall(ctx, t.TempDir(), mod)
	}()

	time.Sleep(50 * time.Millisecond)
	cancel()

	select {
	case err := <-done:
		if err == nil {
			t.Error("expected non-nil error when install is cancelled")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("runInstall did not return within 2s after cancellation")
	}
}

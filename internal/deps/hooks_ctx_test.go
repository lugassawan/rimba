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
	results := RunPostCreateHooks(ctx, dir, []string{"sleep 5", "touch " + marker}, nil)
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
		done <- RunPostCreateHooks(ctx, dir, []string{"sleep 30"}, nil)
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

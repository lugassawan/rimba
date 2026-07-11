package gh

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// withFakeGhOnPath prepends a dummy `gh` executable to PATH for the
// test's lifetime so IsAvailable() returns true regardless of the host.
// The runner is always mocked, so the fake binary is never executed.
func withFakeGhOnPath(t *testing.T) {
	t.Helper()
	writeFakeGh(t, "exit 0\n")
}

// writeFakeGh installs a `gh` script on PATH for the test's lifetime.
// PATH is prepended (not replaced) so other tools stay resolvable.
func writeFakeGh(t *testing.T, script string) {
	t.Helper()
	dir := t.TempDir()
	fake := filepath.Join(dir, "gh")
	if err := os.WriteFile(fake, []byte("#!/bin/sh\n"+script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

// mockRunner implements Runner with a configurable closure for testing.
type mockRunner struct {
	run func(ctx context.Context, args ...string) ([]byte, error)
}

func (m *mockRunner) Run(ctx context.Context, args ...string) ([]byte, error) {
	return m.run(ctx, args...)
}

func assertContains(t *testing.T, err error, substr string) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected error containing %q, got nil", substr)
	}
	if !strings.Contains(err.Error(), substr) {
		t.Errorf("error = %q, want it to contain %q", err, substr)
	}
}

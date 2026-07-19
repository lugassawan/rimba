package operations

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lugassawan/rimba/internal/observability"
)

func TestPostCreateSetupReportsSkippedSymlinks(t *testing.T) {
	tmpDir := t.TempDir()
	wtPath := filepath.Join(tmpDir, "worktree")
	if err := os.MkdirAll(wtPath, 0o755); err != nil {
		t.Fatal(err)
	}

	// .config/ dir with a real file and a nested symlink
	cfgDir := filepath.Join(tmpDir, ".config")
	if err := os.Mkdir(cfgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cfgDir, "real.toml"), []byte("real"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("/dev/null", filepath.Join(cfgDir, "link.toml")); err != nil {
		t.Fatal(err)
	}

	r := &mockRunner{
		run:      func(args ...string) (string, error) { return "", nil },
		runInDir: noopRunInDir,
	}

	result, err := PostCreateSetup(context.Background(), r, PostCreateParams{
		RepoRoot:  tmpDir,
		WtPath:    wtPath,
		Task:      "test-task",
		CopyFiles: []string{".config"},
		SkipDeps:  true,
		SkipHooks: true,
	}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.Copied) != 1 || result.Copied[0] != ".config" {
		t.Errorf("copied = %v, want [.config]", result.Copied)
	}
	wantSymlinks := []string{".config/link.toml"}
	if len(result.SkippedSymlinks) != 1 || result.SkippedSymlinks[0] != wantSymlinks[0] {
		t.Errorf("skippedSymlinks = %v, want %v", result.SkippedSymlinks, wantSymlinks)
	}
}

func TestPostCreateSetupCopiesFiles(t *testing.T) {
	tmpDir := t.TempDir()
	wtPath := filepath.Join(tmpDir, "worktree")
	if err := os.MkdirAll(wtPath, 0o755); err != nil {
		t.Fatal(err)
	}

	// Create a source file to copy
	if err := os.WriteFile(filepath.Join(tmpDir, ".env"), []byte("SECRET=123"), 0o600); err != nil {
		t.Fatal(err)
	}

	r := &mockRunner{
		run:      func(args ...string) (string, error) { return "", nil },
		runInDir: noopRunInDir,
	}

	result, err := PostCreateSetup(context.Background(), r, PostCreateParams{
		RepoRoot:  tmpDir,
		WtPath:    wtPath,
		Task:      "test-task",
		CopyFiles: []string{".env", ".env.local"},
		SkipDeps:  true,
		SkipHooks: true,
	}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.Copied) != 1 || result.Copied[0] != ".env" {
		t.Errorf("copied = %v, want [.env]", result.Copied)
	}
	if len(result.Skipped) != 1 || result.Skipped[0] != ".env.local" {
		t.Errorf("skipped = %v, want [.env.local]", result.Skipped)
	}

	// Verify file was actually copied
	data, err := os.ReadFile(filepath.Join(wtPath, ".env"))
	if err != nil {
		t.Fatalf("copied file not found: %v", err)
	}
	if string(data) != "SECRET=123" {
		t.Errorf("copied file content = %q, want SECRET=123", data)
	}
}

func TestPostCreateSetupSkipDepsAndHooks(t *testing.T) {
	tmpDir := t.TempDir()
	wtPath := filepath.Join(tmpDir, "worktree")
	if err := os.MkdirAll(wtPath, 0o755); err != nil {
		t.Fatal(err)
	}

	r := &mockRunner{
		run:      func(args ...string) (string, error) { return "", nil },
		runInDir: noopRunInDir,
	}

	result, err := PostCreateSetup(context.Background(), r, PostCreateParams{
		RepoRoot:   tmpDir,
		WtPath:     wtPath,
		Task:       "test-task",
		SkipDeps:   true,
		SkipHooks:  true,
		PostCreate: []string{"echo hello"},
	}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.DepsResults != nil {
		t.Errorf("deps should be nil when skipped, got %v", result.DepsResults)
	}
	if result.HookResults != nil {
		t.Errorf("hooks should be nil when skipped, got %v", result.HookResults)
	}
}

func TestPostCreateSetupProgressCallbacks(t *testing.T) {
	tmpDir := t.TempDir()
	wtPath := filepath.Join(tmpDir, "worktree")
	if err := os.MkdirAll(wtPath, 0o755); err != nil {
		t.Fatal(err)
	}

	r := &mockRunner{
		run:      func(args ...string) (string, error) { return "", nil },
		runInDir: noopRunInDir,
	}

	var messages []string
	_, err := PostCreateSetup(context.Background(), r, PostCreateParams{
		RepoRoot:  tmpDir,
		WtPath:    wtPath,
		Task:      "test-task",
		SkipDeps:  true,
		SkipHooks: true,
	}, func(msg string) {
		messages = append(messages, msg)
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(messages) == 0 {
		t.Error("expected at least one progress message")
	}
	if messages[0] != "Copying files..." {
		t.Errorf("first message = %q, want 'Copying files...'", messages[0])
	}
}

func TestPostCreateSetupCopyFilesErrorIncludesRecoveryHint(t *testing.T) {
	tmpDir := t.TempDir()
	wtPath := filepath.Join(tmpDir, "worktree")
	if err := os.MkdirAll(wtPath, 0o755); err != nil {
		t.Fatal(err)
	}

	if os.Getuid() == 0 {
		t.Skip("chmod 000 is ineffective for root; skipping")
	}

	// Create a file then revoke read permissions to force a copy error.
	envPath := filepath.Join(tmpDir, ".env")
	if err := os.WriteFile(envPath, []byte("SECRET=1"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(envPath, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(envPath, 0o600) })

	r := &mockRunner{
		run:      func(args ...string) (string, error) { return "", nil },
		runInDir: noopRunInDir,
	}

	_, err := PostCreateSetup(context.Background(), r, PostCreateParams{
		RepoRoot:  tmpDir,
		WtPath:    wtPath,
		Task:      "test-task",
		CopyFiles: []string{".env"},
		SkipDeps:  true,
		SkipHooks: true,
	}, nil)
	if err == nil {
		t.Fatal("expected error when copying unreadable file")
	}
	if !strings.Contains(err.Error(), "failed to copy files") {
		t.Errorf("error = %q, want to contain 'failed to copy files'", err.Error())
	}
	if !strings.Contains(err.Error(), "To retry, manually copy files to:") {
		t.Errorf("error = %q, want 'To retry, manually copy files to:' context", err.Error())
	}
	if !strings.Contains(err.Error(), "To fix: rimba remove test-task") {
		t.Errorf("error = %q, want recovery hint 'To fix: rimba remove test-task'", err.Error())
	}
}

func TestPostCreateSetupListWorktreesError(t *testing.T) {
	tmpDir := t.TempDir()
	wtPath := filepath.Join(tmpDir, "worktree")
	if err := os.MkdirAll(wtPath, 0o755); err != nil {
		t.Fatal(err)
	}

	r := &mockRunner{
		run: func(args ...string) (string, error) {
			if len(args) > 0 && args[0] == gitCmdWorktree {
				return "", errors.New("permission denied")
			}
			return "", nil
		},
		runInDir: noopRunInDir,
	}

	_, err := PostCreateSetup(context.Background(), r, PostCreateParams{
		RepoRoot: tmpDir,
		WtPath:   wtPath,
		Task:     "test-task",
		SkipDeps: false, // Enable deps so ListWorktrees is called
	}, nil)
	if err == nil {
		t.Fatal("expected error when ListWorktrees fails")
	}
	if !strings.Contains(err.Error(), "failed to list worktrees") {
		t.Errorf("error = %q, want to contain 'failed to list worktrees'", err.Error())
	}
	if !strings.Contains(err.Error(), "To fix: rimba remove test-task") {
		t.Errorf("error = %q, want recovery hint 'To fix: rimba remove test-task'", err.Error())
	}
}

// TestPostCreateSetupRecordsCopyDepsHooksSpans verifies each of the three
// phases records its own named span via the Recorder attached to ctx.
func TestPostCreateSetupRecordsCopyDepsHooksSpans(t *testing.T) {
	tmpDir := t.TempDir()
	wtPath := filepath.Join(tmpDir, "worktree")
	if err := os.MkdirAll(wtPath, 0o755); err != nil {
		t.Fatal(err)
	}

	r := &mockRunner{
		run:      func(args ...string) (string, error) { return "", nil },
		runInDir: noopRunInDir,
	}

	sink := &fakeSink{}
	rec := observability.Maybe(true, sink, "add", "test-task", "", "v1")
	ctx := observability.WithRecorder(context.Background(), rec)

	_, err := PostCreateSetup(ctx, r, PostCreateParams{
		RepoRoot:   tmpDir,
		WtPath:     wtPath,
		Task:       "test-task",
		SkipDeps:   false,
		SkipHooks:  false,
		PostCreate: []string{"echo hello"},
	}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	wantNames := []string{"copy", "deps", "hooks"}
	if len(sink.metrics) != len(wantNames) {
		t.Fatalf("len(sink.metrics) = %d, want %d", len(sink.metrics), len(wantNames))
	}
	for i, want := range wantNames {
		span, ok := sink.metrics[i].(observability.SpanRecord)
		if !ok {
			t.Fatalf("sink.metrics[%d] = %T, want SpanRecord", i, sink.metrics[i])
		}
		if span.Name != want {
			t.Errorf("sink.metrics[%d].Name = %q, want %q", i, span.Name, want)
		}
	}
}

// TestPostCreateSetupRecordsDepsSpanOnListWorktreesError confirms the "deps"
// span's stop() still runs on the early-return error path (ListWorktrees
// failure), not just on success — a span whose stop-closure never runs would
// silently drop that phase's timing.
func TestPostCreateSetupRecordsDepsSpanOnListWorktreesError(t *testing.T) {
	tmpDir := t.TempDir()
	wtPath := filepath.Join(tmpDir, "worktree")
	if err := os.MkdirAll(wtPath, 0o755); err != nil {
		t.Fatal(err)
	}

	r := &mockRunner{
		run: func(args ...string) (string, error) {
			if len(args) > 0 && args[0] == gitCmdWorktree {
				return "", errors.New("permission denied")
			}
			return "", nil
		},
		runInDir: noopRunInDir,
	}

	sink := &fakeSink{}
	rec := observability.Maybe(true, sink, "add", "test-task", "", "v1")
	ctx := observability.WithRecorder(context.Background(), rec)

	_, err := PostCreateSetup(ctx, r, PostCreateParams{
		RepoRoot: tmpDir,
		WtPath:   wtPath,
		Task:     "test-task",
		SkipDeps: false,
	}, nil)
	if err == nil {
		t.Fatal("expected error when ListWorktrees fails")
	}

	var sawDepsSpan bool
	for _, m := range sink.metrics {
		if span, ok := m.(observability.SpanRecord); ok && span.Name == "deps" {
			sawDepsSpan = true
		}
	}
	if !sawDepsSpan {
		t.Error("expected a \"deps\" span even though PostCreateSetup returned early on ListWorktrees error")
	}
}

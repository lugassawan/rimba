package operations

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lugassawan/rimba/internal/metrics"
)

// flushedRun flushes rec to a temp file and unmarshals the single recorded
// run, mirroring internal/metrics' own flush-and-readback test style since
// Recorder has no exported span accessor.
func flushedRun(t *testing.T, rec *metrics.Recorder) metrics.Run {
	t.Helper()
	path := filepath.Join(t.TempDir(), "metrics.jsonl")
	if err := rec.Flush(path, 0); err != nil {
		t.Fatalf("Flush: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	var run metrics.Run
	if err := json.Unmarshal([]byte(strings.TrimSpace(string(data))), &run); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	return run
}

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

func TestPostCreateSetupRecordsSpansInOrder(t *testing.T) {
	tmpDir := t.TempDir()
	wtPath := filepath.Join(tmpDir, "worktree")
	if err := os.MkdirAll(wtPath, 0o755); err != nil {
		t.Fatal(err)
	}

	r := &mockRunner{
		run:      func(args ...string) (string, error) { return "", nil },
		runInDir: noopRunInDir,
	}

	rec := metrics.NewRecorder("add", "test-task", "")

	_, err := PostCreateSetup(context.Background(), r, PostCreateParams{
		RepoRoot:   tmpDir,
		WtPath:     wtPath,
		Task:       "test-task",
		PostCreate: []string{"echo hello"},
		Recorder:   rec,
	}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	run := flushedRun(t, rec)

	// The "hooks" phase span wraps RunPostCreateHooks, whose own per-hook
	// span ("echo hello") is appended before the wrapping span's stop() runs
	// — so alongside copy/deps/hooks we also expect one per-hook span. Filter
	// down to the three phase spans and check their relative order.
	var phaseNames []string
	for _, span := range run.Spans {
		switch span.Name {
		case "copy", "deps", "hooks":
			phaseNames = append(phaseNames, span.Name)
		}
	}

	wantPhaseOrder := []string{"copy", "deps", "hooks"}
	if len(phaseNames) != len(wantPhaseOrder) {
		t.Fatalf("expected phase spans %v, got %v (all spans: %+v)", wantPhaseOrder, phaseNames, run.Spans)
	}
	for i, want := range wantPhaseOrder {
		if phaseNames[i] != want {
			t.Errorf("phase span[%d] = %q, want %q", i, phaseNames[i], want)
		}
	}

	// The hook command itself should also be recorded as its own span.
	var sawHookSpan bool
	for _, span := range run.Spans {
		if span.Name == "echo hello" {
			sawHookSpan = true
		}
	}
	if !sawHookSpan {
		t.Errorf("expected a per-hook span named %q, got %+v", "echo hello", run.Spans)
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

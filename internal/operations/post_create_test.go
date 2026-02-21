package operations

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPostCreateSetup_CopiesFiles(t *testing.T) {
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

	result, err := PostCreateSetup(r, PostCreateParams{
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

func TestPostCreateSetup_SkipDepsAndHooks(t *testing.T) {
	tmpDir := t.TempDir()
	wtPath := filepath.Join(tmpDir, "worktree")
	if err := os.MkdirAll(wtPath, 0o755); err != nil {
		t.Fatal(err)
	}

	r := &mockRunner{
		run:      func(args ...string) (string, error) { return "", nil },
		runInDir: noopRunInDir,
	}

	result, err := PostCreateSetup(r, PostCreateParams{
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

func TestPostCreateSetup_ProgressCallbacks(t *testing.T) {
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
	_, err := PostCreateSetup(r, PostCreateParams{
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

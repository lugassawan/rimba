package operations

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAddWorktree_Success(t *testing.T) {
	tmpDir := t.TempDir()
	wtDir := filepath.Join(tmpDir, ".worktrees")

	r := &mockRunner{
		run: func(args ...string) (string, error) {
			// BranchExists: rev-parse --verify returns error = doesn't exist
			if len(args) > 0 && args[0] == cmdRevParse {
				return "", errors.New("not found")
			}
			// AddWorktree: create the directory
			if len(args) > 0 && args[0] == gitCmdWorktree && len(args) > 1 && args[1] == "add" {
				_ = os.MkdirAll(args[2], 0o755)
				return "", nil
			}
			return "", nil
		},
		runInDir: noopRunInDir,
	}

	result, err := AddWorktree(r, AddParams{
		Task:        "login",
		Prefix:      "feature/",
		Source:      branchMain,
		RepoRoot:    tmpDir,
		WorktreeDir: wtDir,
		SkipDeps:    true,
		SkipHooks:   true,
	}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Task != "login" {
		t.Errorf("expected task 'login', got %q", result.Task)
	}
	if result.Branch != "feature/login" {
		t.Errorf("expected branch 'feature/login', got %q", result.Branch)
	}
	if result.Source != branchMain {
		t.Errorf("expected source 'main', got %q", result.Source)
	}
	if !strings.Contains(result.Path, ".worktrees") {
		t.Errorf("expected path to contain .worktrees, got %q", result.Path)
	}
}

func TestAddWorktree_BranchExists(t *testing.T) {
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			// BranchExists: rev-parse succeeds = branch exists
			if len(args) > 0 && args[0] == cmdRevParse {
				return "abc123", nil
			}
			return "", nil
		},
		runInDir: noopRunInDir,
	}

	_, err := AddWorktree(r, AddParams{
		Task:        "login",
		Prefix:      "feature/",
		Source:      branchMain,
		WorktreeDir: "/tmp/wt",
		SkipDeps:    true,
		SkipHooks:   true,
	}, nil)
	if err == nil {
		t.Fatal("expected error for existing branch")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Errorf("expected 'already exists' error, got: %v", err)
	}
}

func TestAddWorktree_PathExists(t *testing.T) {
	tmpDir := t.TempDir()
	wtDir := filepath.Join(tmpDir, ".worktrees")
	// Create the worktree path so it already exists
	_ = os.MkdirAll(filepath.Join(wtDir, "feature-login"), 0o755)

	r := &mockRunner{
		run: func(args ...string) (string, error) {
			if len(args) > 0 && args[0] == cmdRevParse {
				return "", errors.New("not found")
			}
			return "", nil
		},
		runInDir: noopRunInDir,
	}

	_, err := AddWorktree(r, AddParams{
		Task:        "login",
		Prefix:      "feature/",
		Source:      branchMain,
		RepoRoot:    tmpDir,
		WorktreeDir: wtDir,
		SkipDeps:    true,
		SkipHooks:   true,
	}, nil)
	if err == nil {
		t.Fatal("expected error for existing path")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Errorf("expected 'already exists' error, got: %v", err)
	}
}

func TestAddWorktree_CreateFails(t *testing.T) {
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			if len(args) > 0 && args[0] == cmdRevParse {
				return "", errors.New("not found")
			}
			if len(args) > 0 && args[0] == gitCmdWorktree {
				return "", errors.New("cannot create worktree")
			}
			return "", nil
		},
		runInDir: noopRunInDir,
	}

	_, err := AddWorktree(r, AddParams{
		Task:        "login",
		Prefix:      "feature/",
		Source:      branchMain,
		WorktreeDir: "/tmp/nonexistent-wt",
		SkipDeps:    true,
		SkipHooks:   true,
	}, nil)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "cannot create worktree") {
		t.Errorf("expected create error, got: %v", err)
	}
}

func TestAddWorktree_ProgressCallbacks(t *testing.T) {
	tmpDir := t.TempDir()
	wtDir := filepath.Join(tmpDir, ".worktrees")

	r := &mockRunner{
		run: func(args ...string) (string, error) {
			if len(args) > 0 && args[0] == cmdRevParse {
				return "", errors.New("not found")
			}
			if len(args) > 0 && args[0] == gitCmdWorktree && len(args) > 1 && args[1] == "add" {
				_ = os.MkdirAll(args[2], 0o755)
				return "", nil
			}
			return "", nil
		},
		runInDir: noopRunInDir,
	}

	var messages []string
	progress := ProgressFunc(func(msg string) { messages = append(messages, msg) })

	_, err := AddWorktree(r, AddParams{
		Task:        "login",
		Prefix:      "feature/",
		Source:      branchMain,
		RepoRoot:    tmpDir,
		WorktreeDir: wtDir,
		SkipDeps:    true,
		SkipHooks:   true,
	}, progress)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(messages) < 2 {
		t.Fatalf("expected at least 2 progress messages, got %d: %v", len(messages), messages)
	}
}

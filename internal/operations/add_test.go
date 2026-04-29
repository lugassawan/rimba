package operations

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lugassawan/rimba/internal/progress"
)

func TestAddWorktreeSuccess(t *testing.T) {
	tmpDir := t.TempDir()
	wtDir := filepath.Join(tmpDir, ".worktrees")

	r := &mockRunner{
		run: func(args ...string) (string, error) {
			// BranchExists: rev-parse --verify returns error = doesn't exist
			if len(args) > 0 && args[0] == cmdRevParse {
				return "", errors.New("not found")
			}
			// AddWorktree: create the directory
			if len(args) > 0 && args[0] == gitCmdWorktree && len(args) > 1 && args[1] == gitSubcmdAdd {
				_ = os.MkdirAll(args[2], 0o755)
				return "", nil
			}
			return "", nil
		},
		runInDir: noopRunInDir,
	}

	result, err := AddWorktree(r, AddParams{
		Task:   "login",
		Prefix: "feature/",
		Source: branchMain,
		PostCreateOptions: PostCreateOptions{
			RepoRoot:    tmpDir,
			WorktreeDir: wtDir,
			SkipDeps:    true,
			SkipHooks:   true,
		},
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

func TestAddWorktreeBranchExists(t *testing.T) {
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
		Task:   "login",
		Prefix: "feature/",
		Source: branchMain,
		PostCreateOptions: PostCreateOptions{
			WorktreeDir: "/tmp/wt",
			SkipDeps:    true,
			SkipHooks:   true,
		},
	}, nil)
	if err == nil {
		t.Fatal("expected error for existing branch")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Errorf("expected 'already exists' error, got: %v", err)
	}
}

func TestAddWorktreePathExists(t *testing.T) {
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
		Task:   "login",
		Prefix: "feature/",
		Source: branchMain,
		PostCreateOptions: PostCreateOptions{
			RepoRoot:    tmpDir,
			WorktreeDir: wtDir,
			SkipDeps:    true,
			SkipHooks:   true,
		},
	}, nil)
	if err == nil {
		t.Fatal("expected error for existing path")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Errorf("expected 'already exists' error, got: %v", err)
	}
}

func TestAddWorktreeCreateFails(t *testing.T) {
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
		Task:   "login",
		Prefix: "feature/",
		Source: branchMain,
		PostCreateOptions: PostCreateOptions{
			WorktreeDir: "/tmp/nonexistent-wt",
			SkipDeps:    true,
			SkipHooks:   true,
		},
	}, nil)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "cannot create worktree") {
		t.Errorf("expected create error, got: %v", err)
	}
}

func TestAddWorktreeProgressCallbacks(t *testing.T) {
	tmpDir := t.TempDir()
	wtDir := filepath.Join(tmpDir, ".worktrees")

	r := &mockRunner{
		run: func(args ...string) (string, error) {
			if len(args) > 0 && args[0] == cmdRevParse {
				return "", errors.New("not found")
			}
			if len(args) > 0 && args[0] == gitCmdWorktree && len(args) > 1 && args[1] == gitSubcmdAdd {
				_ = os.MkdirAll(args[2], 0o755)
				return "", nil
			}
			return "", nil
		},
		runInDir: noopRunInDir,
	}

	var messages []string
	onProgress := progress.Func(func(msg string) { messages = append(messages, msg) })

	_, err := AddWorktree(r, AddParams{
		Task:   "login",
		Prefix: "feature/",
		Source: branchMain,
		PostCreateOptions: PostCreateOptions{
			RepoRoot:    tmpDir,
			WorktreeDir: wtDir,
			SkipDeps:    true,
			SkipHooks:   true,
		},
	}, onProgress)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(messages) < 2 {
		t.Fatalf("expected at least 2 progress messages, got %d: %v", len(messages), messages)
	}
}

func TestAddWorktreeWithDeps(t *testing.T) {
	tmpDir := t.TempDir()
	wtDir := filepath.Join(tmpDir, ".worktrees")

	r := &mockRunner{
		run: func(args ...string) (string, error) {
			if len(args) > 0 && args[0] == cmdRevParse {
				return "", errors.New("not found")
			}
			if len(args) > 0 && args[0] == gitCmdWorktree && len(args) > 1 && args[1] == gitSubcmdAdd {
				_ = os.MkdirAll(args[2], 0o755)
				return "", nil
			}
			// ListWorktrees for deps
			if len(args) > 0 && args[0] == gitCmdWorktree && len(args) > 1 && args[1] == cmdList {
				return "worktree " + tmpDir + "\nHEAD abc\nbranch refs/heads/main\n\n", nil
			}
			return "", nil
		},
		runInDir: noopRunInDir,
	}

	result, err := AddWorktree(r, AddParams{
		Task:   "login",
		Prefix: "feature/",
		Source: branchMain,
		PostCreateOptions: PostCreateOptions{
			RepoRoot:    tmpDir,
			WorktreeDir: wtDir,
			SkipDeps:    false,
			AutoDetect:  false,
			SkipHooks:   true,
		},
	}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// DepsResults will be nil because no package.json etc. exists in the tmpDir
	if result.DepsResults != nil {
		t.Errorf("expected nil deps results (no modules), got %v", result.DepsResults)
	}
}

func TestAddWorktreeWithHooks(t *testing.T) {
	tmpDir := t.TempDir()
	wtDir := filepath.Join(tmpDir, ".worktrees")

	r := &mockRunner{
		run: func(args ...string) (string, error) {
			if len(args) > 0 && args[0] == cmdRevParse {
				return "", errors.New("not found")
			}
			if len(args) > 0 && args[0] == gitCmdWorktree && len(args) > 1 && args[1] == gitSubcmdAdd {
				_ = os.MkdirAll(args[2], 0o755)
				return "", nil
			}
			return "", nil
		},
		runInDir: noopRunInDir,
	}

	result, err := AddWorktree(r, AddParams{
		Task:   "login",
		Prefix: "feature/",
		Source: branchMain,
		PostCreateOptions: PostCreateOptions{
			RepoRoot:    tmpDir,
			WorktreeDir: wtDir,
			SkipDeps:    true,
			SkipHooks:   false,
			PostCreate:  []string{"echo hello"},
		},
	}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.HookResults) != 1 {
		t.Fatalf("expected 1 hook result, got %d", len(result.HookResults))
	}
}

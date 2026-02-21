package operations

import (
	"errors"
	"strings"
	"testing"
)

const (
	gitCmdStatus       = "status"
	gitCmdMerge        = "merge"
	branchFeatureLogin = "feature/login"
)

func mergeWorktreeList() string {
	return porcelainEntries(
		struct{ path, branch string }{"/repo", "main"},
		struct{ path, branch string }{"/wt/feature-login", branchFeatureLogin},
		struct{ path, branch string }{"/wt/feature-dashboard", "feature/dashboard"},
	)
}

func mergeRunner(mergeErr error) *mockRunner {
	wt := mergeWorktreeList()
	return &mockRunner{
		run: func(args ...string) (string, error) {
			if len(args) >= 2 && args[0] == gitCmdWorktree {
				return wt, nil
			}
			return "", nil
		},
		runInDir: func(_ string, args ...string) (string, error) {
			if len(args) >= 1 && args[0] == gitCmdStatus {
				return "", nil
			}
			if len(args) >= 1 && args[0] == gitCmdMerge {
				return "", mergeErr
			}
			return "", nil
		},
	}
}

func TestMergeWorktree_MergeToMain(t *testing.T) {
	r := mergeRunner(nil)

	result, err := MergeWorktree(r, MergeParams{
		SourceTask: "login",
		RepoRoot:   "/repo",
		MainBranch: "main",
	}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.SourceBranch != branchFeatureLogin {
		t.Errorf("expected source branch 'feature/login', got %q", result.SourceBranch)
	}
	if result.TargetLabel != "main" {
		t.Errorf("expected target label 'main', got %q", result.TargetLabel)
	}
	if !result.MergingToMain {
		t.Error("expected MergingToMain to be true")
	}
	if !result.SourceRemoved {
		t.Error("expected source to be auto-removed when merging to main")
	}
}

func TestMergeWorktree_MergeToMainKeep(t *testing.T) {
	r := mergeRunner(nil)

	result, err := MergeWorktree(r, MergeParams{
		SourceTask: "login",
		RepoRoot:   "/repo",
		MainBranch: "main",
		Keep:       true,
	}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.SourceRemoved {
		t.Error("expected source NOT to be removed when keep=true")
	}
}

func TestMergeWorktree_MergeToWorktree(t *testing.T) {
	r := mergeRunner(nil)

	result, err := MergeWorktree(r, MergeParams{
		SourceTask: "login",
		IntoTask:   "dashboard",
		RepoRoot:   "/repo",
		MainBranch: "main",
	}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.SourceBranch != branchFeatureLogin {
		t.Errorf("expected source 'feature/login', got %q", result.SourceBranch)
	}
	if result.TargetLabel != "feature/dashboard" {
		t.Errorf("expected target 'feature/dashboard', got %q", result.TargetLabel)
	}
	if result.MergingToMain {
		t.Error("expected MergingToMain to be false")
	}
	if result.SourceRemoved {
		t.Error("expected source NOT to be removed when merging to worktree without delete")
	}
}

func TestMergeWorktree_MergeToWorktreeWithDelete(t *testing.T) {
	r := mergeRunner(nil)

	result, err := MergeWorktree(r, MergeParams{
		SourceTask: "login",
		IntoTask:   "dashboard",
		RepoRoot:   "/repo",
		MainBranch: "main",
		Delete:     true,
	}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.SourceRemoved {
		t.Error("expected source to be removed when delete=true")
	}
}

func TestMergeWorktree_SourceNotFound(t *testing.T) {
	r := mergeRunner(nil)

	_, err := MergeWorktree(r, MergeParams{
		SourceTask: "nonexistent",
		RepoRoot:   "/repo",
		MainBranch: "main",
	}, nil)
	if err == nil {
		t.Fatal("expected error for nonexistent source")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected 'not found' error, got: %v", err)
	}
}

func TestMergeWorktree_TargetNotFound(t *testing.T) {
	r := mergeRunner(nil)

	_, err := MergeWorktree(r, MergeParams{
		SourceTask: "login",
		IntoTask:   "nonexistent",
		RepoRoot:   "/repo",
		MainBranch: "main",
	}, nil)
	if err == nil {
		t.Fatal("expected error for nonexistent target")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected 'not found' error, got: %v", err)
	}
}

func TestMergeWorktree_SourceDirty(t *testing.T) {
	wt := mergeWorktreeList()
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			return wt, nil
		},
		runInDir: func(dir string, args ...string) (string, error) {
			if len(args) >= 1 && args[0] == gitCmdStatus {
				if strings.Contains(dir, "login") {
					return "M dirty.go", nil
				}
				return "", nil
			}
			return "", nil
		},
	}

	_, err := MergeWorktree(r, MergeParams{
		SourceTask: "login",
		RepoRoot:   "/repo",
		MainBranch: "main",
	}, nil)
	if err == nil {
		t.Fatal("expected error for dirty source")
	}
	if !strings.Contains(err.Error(), "uncommitted changes") {
		t.Errorf("expected 'uncommitted changes', got: %v", err)
	}
}

func TestMergeWorktree_TargetDirty(t *testing.T) {
	wt := mergeWorktreeList()
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			return wt, nil
		},
		runInDir: func(dir string, args ...string) (string, error) {
			if len(args) >= 1 && args[0] == gitCmdStatus {
				if dir == "/repo" {
					return "M dirty.go", nil
				}
				return "", nil
			}
			return "", nil
		},
	}

	_, err := MergeWorktree(r, MergeParams{
		SourceTask: "login",
		RepoRoot:   "/repo",
		MainBranch: "main",
	}, nil)
	if err == nil {
		t.Fatal("expected error for dirty target")
	}
	if !strings.Contains(err.Error(), "uncommitted changes") {
		t.Errorf("expected 'uncommitted changes', got: %v", err)
	}
}

func TestMergeWorktree_MergeConflict(t *testing.T) {
	r := mergeRunner(errors.New("conflict"))

	_, err := MergeWorktree(r, MergeParams{
		SourceTask: "login",
		RepoRoot:   "/repo",
		MainBranch: "main",
		Keep:       true, // avoid cleanup attempt
	}, nil)
	if err == nil {
		t.Fatal("expected error from merge conflict")
	}
	if !strings.Contains(err.Error(), "merge failed") {
		t.Errorf("expected 'merge failed', got: %v", err)
	}
}

func TestMergeWorktree_CleanupPartialFailure(t *testing.T) {
	wt := mergeWorktreeList()
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			if len(args) >= 2 && args[0] == gitCmdWorktree && args[1] == "list" {
				return wt, nil
			}
			if len(args) >= 2 && args[0] == gitCmdWorktree && args[1] == "remove" {
				return "", errors.New("worktree locked")
			}
			return "", nil
		},
		runInDir: func(_ string, args ...string) (string, error) {
			if len(args) >= 1 && args[0] == gitCmdStatus {
				return "", nil
			}
			if len(args) >= 1 && args[0] == gitCmdMerge {
				return "", nil
			}
			return "", nil
		},
	}

	result, err := MergeWorktree(r, MergeParams{
		SourceTask: "login",
		RepoRoot:   "/repo",
		MainBranch: "main",
	}, nil)
	if err != nil {
		t.Fatalf("expected no fatal error (cleanup failure is non-fatal), got: %v", err)
	}
	if result.SourceRemoved {
		t.Error("expected source NOT removed when cleanup fails")
	}
	if result.RemoveError == nil {
		t.Error("expected RemoveError to be set")
	}
}

func TestMergeWorktree_ProgressCallbacks(t *testing.T) {
	r := mergeRunner(nil)

	var messages []string
	progress := ProgressFunc(func(msg string) { messages = append(messages, msg) })

	_, err := MergeWorktree(r, MergeParams{
		SourceTask: "login",
		RepoRoot:   "/repo",
		MainBranch: "main",
		Keep:       true,
	}, progress)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(messages) < 2 {
		t.Fatalf("expected at least 2 progress messages, got %d: %v", len(messages), messages)
	}
}

func TestMergeWorktree_ListWorktreesFails(t *testing.T) {
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			return "", errors.New("worktree list failed")
		},
		runInDir: noopRunInDir,
	}

	_, err := MergeWorktree(r, MergeParams{
		SourceTask: "login",
		RepoRoot:   "/repo",
		MainBranch: "main",
	}, nil)
	if err == nil {
		t.Fatal("expected error")
	}
}

package git_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/lugassawan/rimba/internal/git"
	"github.com/lugassawan/rimba/testutil"
)

func TestAddAndListWorktrees(t *testing.T) {
	if testing.Short() {
		t.Skip(skipIntegration)
	}

	repo := testutil.NewTestRepo(t)
	r := &git.ExecRunner{Dir: repo}

	wtPath := filepath.Join(filepath.Dir(repo), "wt-feat-login")

	if err := git.AddWorktree(r, wtPath, "feat/login", "main"); err != nil {
		t.Fatalf(fatalAddWorktree, err)
	}

	// Verify directory exists
	if _, err := os.Stat(wtPath); err != nil {
		t.Fatalf("worktree dir not created: %v", err)
	}

	entries, err := git.ListWorktrees(r)
	if err != nil {
		t.Fatalf("ListWorktrees: %v", err)
	}

	// Should have at least 2: main repo + new worktree
	if len(entries) < 2 {
		t.Fatalf("expected at least 2 worktrees, got %d", len(entries))
	}

	found := false
	for _, e := range entries {
		if e.Branch == "feat/login" {
			found = true
			break
		}
	}
	if !found {
		t.Error("worktree with branch feat/login not found in list")
	}
}

func TestAddWorktreeFromBranch(t *testing.T) {
	if testing.Short() {
		t.Skip(skipIntegration)
	}

	repo := testutil.NewTestRepo(t)
	r := &git.ExecRunner{Dir: repo}

	// Create a branch first
	testutil.GitCmd(t, repo, "branch", "feat/existing")

	wtPath := filepath.Join(filepath.Dir(repo), "wt-from-branch")
	if err := git.AddWorktreeFromBranch(r, wtPath, "feat/existing"); err != nil {
		t.Fatalf("AddWorktreeFromBranch: %v", err)
	}

	if _, err := os.Stat(wtPath); err != nil {
		t.Fatalf("worktree dir not created: %v", err)
	}

	entries, err := git.ListWorktrees(r)
	if err != nil {
		t.Fatalf("ListWorktrees: %v", err)
	}

	found := false
	for _, e := range entries {
		if e.Branch == "feat/existing" {
			found = true
			break
		}
	}
	if !found {
		t.Error("worktree with branch feat/existing not found in list")
	}
}

func TestRemoveWorktree(t *testing.T) {
	if testing.Short() {
		t.Skip(skipIntegration)
	}

	repo := testutil.NewTestRepo(t)
	r := &git.ExecRunner{Dir: repo}

	wtPath := filepath.Join(filepath.Dir(repo), "wt-to-remove")
	if err := git.AddWorktree(r, wtPath, "feat/remove-me", "main"); err != nil {
		t.Fatalf(fatalAddWorktree, err)
	}

	if err := git.RemoveWorktree(r, wtPath, false); err != nil {
		t.Fatalf("RemoveWorktree: %v", err)
	}

	if _, err := os.Stat(wtPath); !os.IsNotExist(err) {
		t.Error("worktree directory should be removed")
	}
}

func TestMoveWorktree(t *testing.T) {
	if testing.Short() {
		t.Skip(skipIntegration)
	}

	repo := testutil.NewTestRepo(t)
	r := &git.ExecRunner{Dir: repo}

	oldPath := filepath.Join(filepath.Dir(repo), "wt-to-move")
	if err := git.AddWorktree(r, oldPath, "feat/move-me", "main"); err != nil {
		t.Fatalf(fatalAddWorktree, err)
	}

	newPath := filepath.Join(filepath.Dir(repo), "wt-moved")
	if err := git.MoveWorktree(r, oldPath, newPath, false); err != nil {
		t.Fatalf("MoveWorktree: %v", err)
	}

	// Old path should be gone
	if _, err := os.Stat(oldPath); !os.IsNotExist(err) {
		t.Error("old worktree directory should be removed")
	}

	// New path should exist
	if _, err := os.Stat(newPath); err != nil {
		t.Errorf("new worktree directory should exist: %v", err)
	}

	// Worktree list should reference the new path.
	// Resolve symlinks because git returns canonical paths (e.g. /private/var on macOS).
	resolvedNew, err := filepath.EvalSymlinks(newPath)
	if err != nil {
		t.Fatalf("EvalSymlinks: %v", err)
	}
	entries, err := git.ListWorktrees(r)
	if err != nil {
		t.Fatalf("ListWorktrees: %v", err)
	}
	found := false
	for _, e := range entries {
		if e.Path == resolvedNew {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected worktree at %s in list, got %v", resolvedNew, entries)
	}
}

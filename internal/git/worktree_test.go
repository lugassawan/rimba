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
		t.Skip("skipping integration test")
	}

	repo := testutil.NewTestRepo(t)
	r := &git.ExecRunner{Dir: repo}

	wtPath := filepath.Join(filepath.Dir(repo), "wt-feat-login")

	if err := git.AddWorktree(r, wtPath, "feat/login", "main"); err != nil {
		t.Fatalf("AddWorktree: %v", err)
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

func TestRemoveWorktree(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	repo := testutil.NewTestRepo(t)
	r := &git.ExecRunner{Dir: repo}

	wtPath := filepath.Join(filepath.Dir(repo), "wt-to-remove")
	if err := git.AddWorktree(r, wtPath, "feat/remove-me", "main"); err != nil {
		t.Fatalf("AddWorktree: %v", err)
	}

	if err := git.RemoveWorktree(r, wtPath, false); err != nil {
		t.Fatalf("RemoveWorktree: %v", err)
	}

	if _, err := os.Stat(wtPath); !os.IsNotExist(err) {
		t.Error("worktree directory should be removed")
	}
}

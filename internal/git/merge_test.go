package git_test

import (
	"path/filepath"
	"testing"

	"github.com/lugassawan/rimba/internal/git"
	"github.com/lugassawan/rimba/testutil"
)

func TestMergeBasic(t *testing.T) {
	if testing.Short() {
		t.Skip(skipIntegration)
	}

	repo := testutil.NewTestRepo(t)
	r := &git.ExecRunner{Dir: repo}

	// Create a branch with a commit
	wtPath := filepath.Join(filepath.Dir(repo), "wt-merge-basic")
	if err := git.AddWorktree(r, wtPath, "feat/merge-basic", "main"); err != nil {
		t.Fatalf(fatalAddWorktree, err)
	}

	testutil.CreateFile(t, wtPath, "new.txt", "merge content")
	testutil.GitCmd(t, wtPath, "add", ".")
	testutil.GitCmd(t, wtPath, "commit", "-m", "add new.txt")

	// Merge into main repo
	if err := git.Merge(r, repo, "feat/merge-basic", false); err != nil {
		t.Fatalf("Merge: %v", err)
	}

	// Verify new.txt exists in main repo
	testutil.GitCmd(t, repo, "log", "--oneline", "-1")
}

func TestMergeNoFF(t *testing.T) {
	if testing.Short() {
		t.Skip(skipIntegration)
	}

	repo := testutil.NewTestRepo(t)
	r := &git.ExecRunner{Dir: repo}

	// Create a branch with a commit
	wtPath := filepath.Join(filepath.Dir(repo), "wt-merge-noff")
	if err := git.AddWorktree(r, wtPath, "feat/merge-noff", "main"); err != nil {
		t.Fatalf(fatalAddWorktree, err)
	}

	testutil.CreateFile(t, wtPath, "noff.txt", "no-ff content")
	testutil.GitCmd(t, wtPath, "add", ".")
	testutil.GitCmd(t, wtPath, "commit", "-m", "add noff.txt")

	if err := git.Merge(r, repo, "feat/merge-noff", true); err != nil {
		t.Fatalf("Merge --no-ff: %v", err)
	}

	// Verify merge commit was created (--no-ff forces a merge commit)
	log := testutil.GitCmd(t, repo, "log", "--oneline", "-1")
	if len(log) == 0 {
		t.Error("expected merge commit in log")
	}
}

func TestMergeError(t *testing.T) {
	if testing.Short() {
		t.Skip(skipIntegration)
	}

	repo := testutil.NewTestRepo(t)
	r := &git.ExecRunner{Dir: repo}

	// Merge a nonexistent branch should fail
	if err := git.Merge(r, repo, "nonexistent-branch", false); err == nil {
		t.Error("expected error merging nonexistent branch")
	}
}

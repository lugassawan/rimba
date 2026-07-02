package git_test

import (
	"context"
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
	if err := git.AddWorktree(context.Background(), r, wtPath, "feat/merge-basic", "main"); err != nil {
		t.Fatalf(fatalAddWorktree, err)
	}

	testutil.CreateFile(t, wtPath, "new.txt", "merge content")
	testutil.GitCmd(t, wtPath, "add", ".")
	testutil.GitCmd(t, wtPath, "commit", "-m", "add new.txt")

	// Merge into main repo
	if err := git.Merge(context.Background(), r, repo, "feat/merge-basic", false); err != nil {
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
	if err := git.AddWorktree(context.Background(), r, wtPath, "feat/merge-noff", "main"); err != nil {
		t.Fatalf(fatalAddWorktree, err)
	}

	testutil.CreateFile(t, wtPath, "noff.txt", "no-ff content")
	testutil.GitCmd(t, wtPath, "add", ".")
	testutil.GitCmd(t, wtPath, "commit", "-m", "add noff.txt")

	if err := git.Merge(context.Background(), r, repo, "feat/merge-noff", true); err != nil {
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
	if err := git.Merge(context.Background(), r, repo, "nonexistent-branch", false); err == nil {
		t.Error("expected error merging nonexistent branch")
	}
}

func TestMergeInProgress(t *testing.T) {
	if testing.Short() {
		t.Skip(skipIntegration)
	}

	repo := testutil.NewTestRepo(t)
	r := &git.ExecRunner{Dir: repo}

	// Clean repo: no merge in progress
	inProgress, err := git.MergeInProgress(context.Background(), r, repo)
	if err != nil {
		t.Fatalf("MergeInProgress on clean repo: %v", err)
	}
	if inProgress {
		t.Error("expected MergeInProgress=false on clean repo")
	}

	// Set up a conflicting merge: two branches edit the same file differently.
	wtPath := filepath.Join(filepath.Dir(repo), "wt-merge-in-progress")
	if err := git.AddWorktree(context.Background(), r, wtPath, "feat/conflict-source", "main"); err != nil {
		t.Fatalf(fatalAddWorktree, err)
	}
	testutil.CreateFile(t, wtPath, "conflict.txt", "source version")
	testutil.GitCmd(t, wtPath, "add", ".")
	testutil.GitCmd(t, wtPath, "commit", "-m", "source change")

	// Also add the file on main with different content so it conflicts.
	testutil.CreateFile(t, repo, "conflict.txt", "main version")
	testutil.GitCmd(t, repo, "add", ".")
	testutil.GitCmd(t, repo, "commit", "-m", "main change")

	// Attempt the conflicting merge — it should fail.
	if err := git.Merge(context.Background(), r, repo, "feat/conflict-source", false); err == nil {
		t.Fatal("expected conflict error from Merge")
	}

	// Now a merge should be in progress.
	inProgress, err = git.MergeInProgress(context.Background(), r, repo)
	if err != nil {
		t.Fatalf("MergeInProgress after conflict: %v", err)
	}
	if !inProgress {
		t.Error("expected MergeInProgress=true after conflicting merge")
	}
}

func TestMergeAbort(t *testing.T) {
	if testing.Short() {
		t.Skip(skipIntegration)
	}

	repo := testutil.NewTestRepo(t)
	r := &git.ExecRunner{Dir: repo}

	// Set up a conflicting merge (same steps as TestMergeInProgress).
	wtPath := filepath.Join(filepath.Dir(repo), "wt-merge-abort")
	if err := git.AddWorktree(context.Background(), r, wtPath, "feat/abort-source", "main"); err != nil {
		t.Fatalf(fatalAddWorktree, err)
	}
	testutil.CreateFile(t, wtPath, "conflict.txt", "source version")
	testutil.GitCmd(t, wtPath, "add", ".")
	testutil.GitCmd(t, wtPath, "commit", "-m", "source change")

	testutil.CreateFile(t, repo, "conflict.txt", "main version")
	testutil.GitCmd(t, repo, "add", ".")
	testutil.GitCmd(t, repo, "commit", "-m", "main change")

	if err := git.Merge(context.Background(), r, repo, "feat/abort-source", false); err == nil {
		t.Fatal("expected conflict error from Merge")
	}

	// Abort the merge.
	if err := git.MergeAbort(r, repo); err != nil {
		t.Fatalf("MergeAbort: %v", err)
	}

	// Repo should be clean and no merge in progress.
	dirty, err := git.IsDirty(context.Background(), r, repo)
	if err != nil {
		t.Fatalf("IsDirty after abort: %v", err)
	}
	if dirty {
		t.Error("expected repo to be clean after MergeAbort")
	}
	inProgress, err := git.MergeInProgress(context.Background(), r, repo)
	if err != nil {
		t.Fatalf("MergeInProgress after abort: %v", err)
	}
	if inProgress {
		t.Error("expected MergeInProgress=false after MergeAbort")
	}
}

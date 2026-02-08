package git_test

import (
	"testing"

	"github.com/lugassawan/rimba/internal/git"
	"github.com/lugassawan/rimba/testutil"
)

const (
	skipIntegration  = "skipping integration test"
	fatalAddWorktree = "AddWorktree: %v"
	branchToDelete   = "to-delete"
)

func TestBranchExists(t *testing.T) {
	if testing.Short() {
		t.Skip(skipIntegration)
	}

	repo := testutil.NewTestRepo(t)
	r := &git.ExecRunner{Dir: repo}

	if !git.BranchExists(r, "main") {
		t.Error("expected main branch to exist")
	}

	if git.BranchExists(r, "nonexistent") {
		t.Error("expected nonexistent branch to not exist")
	}
}

func TestDeleteBranch(t *testing.T) {
	if testing.Short() {
		t.Skip(skipIntegration)
	}

	repo := testutil.NewTestRepo(t)
	r := &git.ExecRunner{Dir: repo}

	// Create a branch
	testutil.GitCmd(t, repo, "branch", branchToDelete)

	if !git.BranchExists(r, branchToDelete) {
		t.Fatal("branch should exist before delete")
	}

	if err := git.DeleteBranch(r, branchToDelete, false); err != nil {
		t.Fatalf("DeleteBranch: %v", err)
	}

	if git.BranchExists(r, branchToDelete) {
		t.Error("branch should not exist after delete")
	}
}

func TestIsDirty(t *testing.T) {
	if testing.Short() {
		t.Skip(skipIntegration)
	}

	repo := testutil.NewTestRepo(t)
	r := &git.ExecRunner{Dir: repo}

	dirty, err := git.IsDirty(r, repo)
	if err != nil {
		t.Fatalf("IsDirty: %v", err)
	}
	if dirty {
		t.Error("clean repo should not be dirty")
	}

	// Make it dirty
	testutil.CreateFile(t, repo, "new.txt", "dirty")

	dirty, err = git.IsDirty(r, repo)
	if err != nil {
		t.Fatalf("IsDirty: %v", err)
	}
	if !dirty {
		t.Error("repo with untracked file should be dirty")
	}
}

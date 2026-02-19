package git_test

import (
	"slices"
	"testing"
	"time"

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

func TestLastCommitTime(t *testing.T) {
	if testing.Short() {
		t.Skip(skipIntegration)
	}

	repo := testutil.NewTestRepo(t)
	r := &git.ExecRunner{Dir: repo}

	ct, err := git.LastCommitTime(r, "main")
	if err != nil {
		t.Fatalf("LastCommitTime: %v", err)
	}

	// Should be within the last minute (just created)
	if time.Since(ct) > time.Minute {
		t.Errorf("commit time %v is too old", ct)
	}
}

func TestLastCommitInfo(t *testing.T) {
	if testing.Short() {
		t.Skip(skipIntegration)
	}

	repo := testutil.NewTestRepo(t)
	r := &git.ExecRunner{Dir: repo}

	ct, subject, err := git.LastCommitInfo(r, "main")
	if err != nil {
		t.Fatalf("LastCommitInfo: %v", err)
	}

	if time.Since(ct) > time.Minute {
		t.Errorf("commit time %v is too old", ct)
	}
	if subject == "" {
		t.Error("expected non-empty commit subject")
	}
}

func TestLastCommitInfoError(t *testing.T) {
	if testing.Short() {
		t.Skip(skipIntegration)
	}

	repo := testutil.NewTestRepo(t)
	r := &git.ExecRunner{Dir: repo}

	_, _, err := git.LastCommitInfo(r, "nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent branch")
	}
}

func TestLastCommitTimeError(t *testing.T) {
	if testing.Short() {
		t.Skip(skipIntegration)
	}

	repo := testutil.NewTestRepo(t)
	r := &git.ExecRunner{Dir: repo}

	_, err := git.LastCommitTime(r, "nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent branch")
	}
}

func TestLocalBranches(t *testing.T) {
	if testing.Short() {
		t.Skip(skipIntegration)
	}

	repo := testutil.NewTestRepo(t)
	r := &git.ExecRunner{Dir: repo}

	branches, err := git.LocalBranches(r)
	if err != nil {
		t.Fatalf("LocalBranches: %v", err)
	}

	if len(branches) == 0 {
		t.Fatal("expected at least one branch")
	}

	if !slices.Contains(branches, "main") {
		t.Errorf("expected 'main' in branches, got %v", branches)
	}
}

func TestLocalBranchesMultiple(t *testing.T) {
	if testing.Short() {
		t.Skip(skipIntegration)
	}

	repo := testutil.NewTestRepo(t)
	r := &git.ExecRunner{Dir: repo}

	testutil.GitCmd(t, repo, "branch", "feature/test-branch")

	branches, err := git.LocalBranches(r)
	if err != nil {
		t.Fatalf("LocalBranches: %v", err)
	}

	if len(branches) < 2 {
		t.Fatalf("expected at least 2 branches, got %d: %v", len(branches), branches)
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

func TestIsSquashMergedIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip(skipIntegration)
	}

	repo := testutil.NewTestRepo(t)
	r := &git.ExecRunner{Dir: repo}

	// Create a feature branch with a commit
	testutil.GitCmd(t, repo, "checkout", "-b", "feature/squash-test")
	testutil.CreateFile(t, repo, "squash.txt", "squash content")
	testutil.GitCmd(t, repo, "add", ".")
	testutil.GitCmd(t, repo, "commit", "-m", "feature commit")

	// Go back to main and squash-merge
	testutil.GitCmd(t, repo, "checkout", "main")
	testutil.GitCmd(t, repo, "merge", "--squash", "feature/squash-test")
	testutil.GitCmd(t, repo, "commit", "-m", "squash merge feature")

	// The branch should be detected as squash-merged
	merged, err := git.IsSquashMerged(r, "main", "feature/squash-test")
	if err != nil {
		t.Fatalf("IsSquashMerged: %v", err)
	}
	if !merged {
		t.Error("expected squash-merged branch to be detected")
	}
}

func TestIsSquashMergedIntegrationNotMerged(t *testing.T) {
	if testing.Short() {
		t.Skip(skipIntegration)
	}

	repo := testutil.NewTestRepo(t)
	r := &git.ExecRunner{Dir: repo}

	// Create a feature branch with a commit but don't merge it
	testutil.GitCmd(t, repo, "checkout", "-b", "feature/unmerged")
	testutil.CreateFile(t, repo, "unmerged.txt", "unmerged content")
	testutil.GitCmd(t, repo, "add", ".")
	testutil.GitCmd(t, repo, "commit", "-m", "unmerged commit")

	testutil.GitCmd(t, repo, "checkout", "main")

	merged, err := git.IsSquashMerged(r, "main", "feature/unmerged")
	if err != nil {
		t.Fatalf("IsSquashMerged: %v", err)
	}
	if merged {
		t.Error("expected unmerged branch to not be detected as squash-merged")
	}
}

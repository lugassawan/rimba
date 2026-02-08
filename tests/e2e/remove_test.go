package e2e_test

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lugassawan/rimba/internal/resolver"
	"github.com/lugassawan/rimba/testutil"
)

func TestRemoveRemovesWorktree(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupInitializedRepo(t)
	rimbaSuccess(t, repo, "add", taskRm)

	r := rimbaSuccess(t, repo, "remove", taskRm)
	assertContains(t, r.Stdout, "Removed worktree")

	// Verify the worktree directory is gone
	cfg := loadConfig(t, repo)
	wtDir := filepath.Join(repo, cfg.WorktreeDir)
	branch := resolver.BranchName(defaultPrefix, taskRm)
	wtPath := resolver.WorktreePath(wtDir, branch)
	assertFileNotExists(t, wtPath)
}

func TestRemoveWithBranchFlag(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupInitializedRepo(t)
	rimbaSuccess(t, repo, "add", taskRmBranch)

	r := rimbaSuccess(t, repo, "remove", "--branch", taskRmBranch)
	assertContains(t, r.Stdout, "Removed worktree")
	assertContains(t, r.Stdout, "Deleted branch")

	// Verify branch is deleted
	out := testutil.GitCmd(t, repo, "branch", "--list")
	if strings.Contains(out, defaultPrefix+taskRmBranch) {
		t.Errorf("expected branch %s%s to be deleted, but it still exists", defaultPrefix, taskRmBranch)
	}
}

func TestRemovePreservesBranch(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupInitializedRepo(t)
	rimbaSuccess(t, repo, "add", taskKeepBranch)

	rimbaSuccess(t, repo, "remove", taskKeepBranch)

	// Verify branch still exists
	out := testutil.GitCmd(t, repo, "branch", "--list")
	if !strings.Contains(out, defaultPrefix+taskKeepBranch) {
		t.Errorf("expected branch %s%s to still exist", defaultPrefix, taskKeepBranch)
	}
}

func TestRemoveForceFlag(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupInitializedRepo(t)
	rimbaSuccess(t, repo, "add", taskForce)

	// Make the worktree dirty
	cfg := loadConfig(t, repo)
	wtDir := filepath.Join(repo, cfg.WorktreeDir)
	branch := resolver.BranchName(defaultPrefix, taskForce)
	wtPath := resolver.WorktreePath(wtDir, branch)
	testutil.CreateFile(t, wtPath, "dirty.txt", "uncommitted changes")

	// Stage the file to make git worktree remove fail without force
	cmd := exec.Command("git", "add", "dirty.txt")
	cmd.Dir = wtPath
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git add: %s: %v", out, err)
	}

	rimbaSuccess(t, repo, "remove", "-f", taskForce)
	assertFileNotExists(t, wtPath)
}

func TestRemovePartialFailBranchHint(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupInitializedRepo(t)
	rimbaSuccess(t, repo, "add", taskRmPartial)

	// Commit in the worktree so the branch has unmerged changes,
	// causing git branch -d to refuse deletion after worktree removal.
	cfg := loadConfig(t, repo)
	wtDir := filepath.Join(repo, cfg.WorktreeDir)
	branch := resolver.BranchName(defaultPrefix, taskRmPartial)
	wtPath := resolver.WorktreePath(wtDir, branch)

	testutil.CreateFile(t, wtPath, "unmerged.txt", "content")
	testutil.GitCmd(t, wtPath, "add", ".")
	testutil.GitCmd(t, wtPath, "commit", "-m", "unmerged commit")

	r := rimbaFail(t, repo, "remove", "--branch", taskRmPartial)
	assertContains(t, r.Stderr, "worktree removed but failed to delete branch")
	assertContains(t, r.Stderr, "git branch -d")
	assertContains(t, r.Stderr, "-D to force")
}

func TestRemoveFailsNonexistent(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupInitializedRepo(t)
	r := rimbaFail(t, repo, "remove", "ghost-task")
	assertContains(t, r.Stderr, "not found")
}

func TestRemoveFailsNoArgs(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupInitializedRepo(t)
	rimbaFail(t, repo, "remove")
}

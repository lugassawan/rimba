package e2e_test

import (
	"os"
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
	assertContains(t, r.Stdout, msgRemovedWorktree)
	assertContains(t, r.Stdout, msgDeletedBranch)

	// Verify the worktree directory is gone
	cfg := loadConfig(t, repo)
	wtDir := filepath.Join(repo, cfg.WorktreeDir)
	branch := resolver.BranchName(defaultPrefix, taskRm)
	wtPath := resolver.WorktreePath(wtDir, branch)
	assertFileNotExists(t, wtPath)

	// Verify branch is deleted
	out := testutil.GitCmd(t, repo, "branch", flagBranchList)
	if strings.Contains(out, defaultPrefix+taskRm) {
		t.Errorf("expected branch %s%s to be deleted, but it still exists", defaultPrefix, taskRm)
	}
}

func TestRemoveKeepBranchFlag(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupInitializedRepo(t)
	rimbaSuccess(t, repo, "add", taskKeepBranch)

	rimbaSuccess(t, repo, "remove", "--keep-branch", taskKeepBranch)

	// Verify branch still exists
	out := testutil.GitCmd(t, repo, "branch", flagBranchList)
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

	r := rimbaSuccess(t, repo, "remove", "-f", taskForce)
	assertFileNotExists(t, wtPath)
	assertContains(t, r.Stdout, msgDeletedBranch)
}

func TestRemoveDeletesUnmergedBranch(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupInitializedRepo(t)
	rimbaSuccess(t, repo, "add", taskRmPartial)

	// Commit in the worktree so the branch has unmerged changes
	cfg := loadConfig(t, repo)
	wtDir := filepath.Join(repo, cfg.WorktreeDir)
	branch := resolver.BranchName(defaultPrefix, taskRmPartial)
	wtPath := resolver.WorktreePath(wtDir, branch)

	testutil.CreateFile(t, wtPath, "unmerged.txt", "content")
	testutil.GitCmd(t, wtPath, "add", ".")
	testutil.GitCmd(t, wtPath, "commit", "-m", "unmerged commit")

	r := rimbaSuccess(t, repo, "remove", taskRmPartial)
	assertContains(t, r.Stdout, msgRemovedWorktree)
	assertContains(t, r.Stdout, msgDeletedBranch)

	// Verify branch is gone (force delete handles unmerged)
	out := testutil.GitCmd(t, repo, "branch", flagBranchList)
	if strings.Contains(out, defaultPrefix+taskRmPartial) {
		t.Errorf("expected branch %s%s to be deleted, but it still exists", defaultPrefix, taskRmPartial)
	}
}

// TestRemovePrunableWorktreeRecovers: an orphaned worktree (dir present, .git
// deleted out-of-band) must be healed via repair and fully removed, not just deregistered.
func TestRemovePrunableWorktreeRecovers(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupInitializedRepo(t)
	rimbaSuccess(t, repo, "add", taskPrunableRemove)

	cfg := loadConfig(t, repo)
	wtDir := filepath.Join(repo, cfg.WorktreeDir)
	branch := resolver.BranchName(defaultPrefix, taskPrunableRemove)
	wtPath := resolver.WorktreePath(wtDir, branch)

	if err := os.Remove(filepath.Join(wtPath, ".git")); err != nil {
		t.Fatalf("remove .git file: %v", err)
	}

	r := rimbaSuccess(t, repo, "remove", taskPrunableRemove)
	assertContains(t, r.Stdout, msgRemovedWorktree)
	assertContains(t, r.Stdout, msgDeletedBranch)
	assertNotContains(t, r.Stdout, "Failed to remove")

	// The stale worktree admin entry must be gone from git's own bookkeeping.
	out := testutil.GitCmd(t, repo, "worktree", "list")
	if strings.Contains(out, wtPath) {
		t.Errorf("expected worktree entry for %s to be removed, got: %s", wtPath, out)
	}

	// Unlike a prune-only recovery, a healed orphan leaves nothing on disk.
	if _, err := os.Stat(wtPath); !os.IsNotExist(err) {
		t.Errorf("expected worktree directory %s to be fully removed, stat err: %v", wtPath, err)
	}

	// Branch must still be deleted despite the broken .git file.
	branches := testutil.GitCmd(t, repo, "branch", flagBranchList)
	if strings.Contains(branches, branch) {
		t.Errorf("expected branch %s to be deleted, but it still exists", branch)
	}
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

func TestRemoveDryRun(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupInitializedRepo(t)
	rimbaSuccess(t, repo, "add", taskRm, flagSkipDepsE2E, flagSkipHooksE2E)

	cfg := loadConfig(t, repo)
	wtDir := filepath.Join(repo, cfg.WorktreeDir)
	branch := resolver.BranchName(defaultPrefix, taskRm)
	wtPath := resolver.WorktreePath(wtDir, branch)

	r := rimbaSuccess(t, repo, "remove", taskRm, flagDryRunE2E)
	assertContains(t, r.Stdout, "[dry-run]")

	// Worktree must still exist after dry-run
	assertFileExists(t, wtPath)

	// Branch must still exist
	out := testutil.GitCmd(t, repo, "branch", flagBranchList)
	if !strings.Contains(out, defaultPrefix+taskRm) {
		t.Errorf("expected branch %s%s to still exist after dry-run", defaultPrefix, taskRm)
	}
}

package e2e_test

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lugassawan/rimba/internal/resolver"
	"github.com/lugassawan/rimba/testutil"
)

const (
	taskRename     = "rename-old"
	taskRenameNew  = "rename-new"
	taskRenameLock = "rename-lock"
	taskLockNew    = "lock-new"
	taskGhostRn    = "ghost-rename"
	taskGhostNew   = "ghost-new"
	taskBrExist    = "br-exist-old"
	taskBrExistNew = "br-exist-new"
)

func TestRenameRenamesWorktree(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupInitializedRepo(t)
	rimbaSuccess(t, repo, "add", taskRename)

	cfg := loadConfig(t, repo)
	wtDir := filepath.Join(repo, cfg.WorktreeDir)
	oldBranch := resolver.BranchName(defaultPrefix, taskRename)
	oldPath := resolver.WorktreePath(wtDir, oldBranch)
	newBranch := resolver.BranchName(defaultPrefix, taskRenameNew)
	newPath := resolver.WorktreePath(wtDir, newBranch)

	r := rimbaSuccess(t, repo, "rename", taskRename, taskRenameNew)
	assertContains(t, r.Stdout, "Renamed worktree")

	// Old directory and branch should be gone
	assertFileNotExists(t, oldPath)
	branches := testutil.GitCmd(t, repo, "branch", "--list")
	if strings.Contains(branches, oldBranch) {
		t.Errorf("expected old branch %q to be gone", oldBranch)
	}

	// New directory and branch should exist
	assertFileExists(t, newPath)
	if !strings.Contains(branches, newBranch) {
		t.Errorf("expected new branch %q to exist", newBranch)
	}
}

func TestRenamePreservesPrefix(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupInitializedRepo(t)
	rimbaSuccess(t, repo, "add", "--bugfix", "old-bug")

	r := rimbaSuccess(t, repo, "rename", "old-bug", "new-bug")
	assertContains(t, r.Stdout, "Renamed worktree")

	// Verify the branch preserved the bugfix/ prefix
	branches := testutil.GitCmd(t, repo, "branch", "--list")
	if !strings.Contains(branches, "bugfix/new-bug") {
		t.Errorf("expected branch bugfix/new-bug to exist, got branches:\n%s", branches)
	}
	if strings.Contains(branches, "feature/new-bug") {
		t.Errorf("did not expect branch feature/new-bug")
	}
}

func TestRenameForceFlag(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupInitializedRepo(t)
	rimbaSuccess(t, repo, "add", taskRenameLock)

	cfg := loadConfig(t, repo)
	wtDir := filepath.Join(repo, cfg.WorktreeDir)
	branch := resolver.BranchName(defaultPrefix, taskRenameLock)
	wtPath := resolver.WorktreePath(wtDir, branch)

	// Lock the worktree so rename fails without --force
	cmd := exec.Command("git", "worktree", "lock", wtPath)
	cmd.Dir = repo
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git worktree lock: %s: %v", out, err)
	}

	// Should fail without --force
	rimbaFail(t, repo, "rename", taskRenameLock, taskLockNew)

	// Should succeed with --force
	newBranch := resolver.BranchName(defaultPrefix, taskLockNew)
	newPath := resolver.WorktreePath(wtDir, newBranch)

	rimbaSuccess(t, repo, "rename", "-f", taskRenameLock, taskLockNew)
	assertFileNotExists(t, wtPath)
	assertFileExists(t, newPath)
}

func TestRenameFailsNonexistent(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupInitializedRepo(t)
	r := rimbaFail(t, repo, "rename", taskGhostRn, taskGhostNew)
	assertContains(t, r.Stderr, "not found")
}

func TestRenameFailsBranchExists(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupInitializedRepo(t)
	rimbaSuccess(t, repo, "add", taskBrExist)
	rimbaSuccess(t, repo, "add", taskBrExistNew)

	r := rimbaFail(t, repo, "rename", taskBrExist, taskBrExistNew)
	assertContains(t, r.Stderr, "already exists")
}

func TestRenameFailsNoArgs(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupInitializedRepo(t)
	rimbaFail(t, repo, "rename")
}

func TestRenameFailsOneArg(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupInitializedRepo(t)
	rimbaFail(t, repo, "rename", "some-task")
}

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
	branches := testutil.GitCmd(t, repo, "branch", flagBranchList)
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
	branches := testutil.GitCmd(t, repo, "branch", flagBranchList)
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
	assertContains(t, r.Stderr, "To fix:")
	assertContains(t, r.Stderr, "git branch -D")
}

func TestRenamePartialFailRollback(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupInitializedRepo(t)
	rimbaSuccess(t, repo, "add", "--bugfix", "rn-partial-old")

	cfg := loadConfig(t, repo)
	wtDir := filepath.Join(repo, cfg.WorktreeDir)
	oldBranch := resolver.BranchName(bugfixPrefix, "rn-partial-old")
	oldPath := resolver.WorktreePath(wtDir, oldBranch)
	newBranch := resolver.BranchName(bugfixPrefix, "rn-partial-new")
	newPath := resolver.WorktreePath(wtDir, newBranch)

	// Create a sub-branch that blocks the rename target in the git ref namespace.
	// "bugfix/rn-partial-new/sub" makes bugfix/rn-partial-new a directory in
	// .git/refs/heads/, so git branch -m cannot create it as a file.
	testutil.GitCmd(t, repo, "branch", "bugfix/rn-partial-new/sub")

	r := rimbaFail(t, repo, "rename", "rn-partial-old", "rn-partial-new")

	// Error should report the branch rename failure and successful rollback.
	assertContains(t, r.Stderr, "failed to rename branch")
	assertContains(t, r.Stderr, "moved back")
	assertContains(t, r.Stderr, "To fix:")
	assertContains(t, r.Stderr, "git branch -m")

	// Worktree should be back at its original path (rollback succeeded).
	assertFileExists(t, oldPath)
	assertFileNotExists(t, newPath)
}

func TestRenameFailsNoArgs(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupInitializedRepo(t)
	rimbaFail(t, repo, "rename")
}

func TestRenameNoOpReportsCleanly(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	// 1-arg invocation with no prefix flag — same task, same prefix → no-op error.
	repo := setupInitializedRepo(t)
	rimbaSuccess(t, repo, "add", "retype-noop")

	r := rimbaFail(t, repo, "rename", "retype-noop")
	assertContains(t, r.Stderr, "nothing to change")
}

func TestRenameRetypeOnly(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupInitializedRepo(t)
	rimbaSuccess(t, repo, "add", "retype-auth")

	cfg := loadConfig(t, repo)
	wtDir := filepath.Join(repo, cfg.WorktreeDir)

	r := rimbaSuccess(t, repo, "rename", "retype-auth", "--bugfix")
	assertContains(t, r.Stdout, "feature/retype-auth -> bugfix/retype-auth")

	// Old branch and directory gone
	assertFileNotExists(t, resolver.WorktreePath(wtDir, "feature/retype-auth"))
	branches := testutil.GitCmd(t, repo, "branch", flagBranchList)
	if strings.Contains(branches, "feature/retype-auth") {
		t.Errorf("expected feature/retype-auth to be gone")
	}

	// New branch and directory exist
	assertFileExists(t, resolver.WorktreePath(wtDir, "bugfix/retype-auth"))
	if !strings.Contains(branches, "bugfix/retype-auth") {
		t.Errorf("expected bugfix/retype-auth branch to exist, got:\n%s", branches)
	}
}

func TestRenameTaskAndType(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupInitializedRepo(t)
	rimbaSuccess(t, repo, "add", "retype-src")

	cfg := loadConfig(t, repo)
	wtDir := filepath.Join(repo, cfg.WorktreeDir)

	r := rimbaSuccess(t, repo, "rename", "retype-src", "retype-dst", "--bugfix")
	assertContains(t, r.Stdout, "feature/retype-src -> bugfix/retype-dst")

	assertFileNotExists(t, resolver.WorktreePath(wtDir, "feature/retype-src"))
	assertFileExists(t, resolver.WorktreePath(wtDir, "bugfix/retype-dst"))

	branches := testutil.GitCmd(t, repo, "branch", flagBranchList)
	if strings.Contains(branches, "feature/retype-src") {
		t.Errorf("expected feature/retype-src to be gone after rename, got:\n%s", branches)
	}
	if !strings.Contains(branches, "bugfix/retype-dst") {
		t.Errorf("expected bugfix/retype-dst branch to exist, got:\n%s", branches)
	}
}

func TestRenamePostRenameHookRuns(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupInitializedRepo(t)
	cfg := loadConfig(t, repo)
	marker := filepath.Join(repo, "hook-ran.txt")
	cfg.PostRename = []string{"touch " + marker}
	saveConfig(t, repo, cfg)

	rimbaWithEnv(t, repo, []string{"RIMBA_TRUST_YES=1"}, "add", "hook-src", flagSkipDepsE2E, flagSkipHooksE2E)
	rimbaWithEnv(t, repo, []string{"RIMBA_TRUST_YES=1"}, "rename", "hook-src", "hook-dst", flagSkipDepsE2E)

	assertFileExists(t, marker)
}

func TestRenameSkipHooksSkipsPostRenameHook(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupInitializedRepo(t)
	cfg := loadConfig(t, repo)
	marker := filepath.Join(repo, "hook-ran.txt")
	cfg.PostRename = []string{"touch " + marker}
	saveConfig(t, repo, cfg)

	rimbaWithEnv(t, repo, []string{"RIMBA_TRUST_YES=1"}, "add", "hook-skip-src", flagSkipDepsE2E, flagSkipHooksE2E)
	rimbaWithEnv(t, repo, []string{"RIMBA_TRUST_YES=1"}, "rename", "hook-skip-src", "hook-skip-dst",
		flagSkipDepsE2E, flagSkipHooksE2E)

	assertFileNotExists(t, marker)
}

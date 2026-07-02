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

const (
	taskRename      = "rename-old"
	taskRenameNew   = "rename-new"
	taskRenameLock  = "rename-lock"
	taskLockNew     = "lock-new"
	taskGhostRn     = "ghost-rename"
	taskGhostNew    = "ghost-new"
	taskBrExist     = "br-exist-old"
	taskBrExistNew  = "br-exist-new"
	taskRenamePush  = "rn-push-src"
	taskRenamePush2 = "rn-push-dst"
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

func TestRenameNoOpExplicitSameType(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	// Explicit --bugfix on an already-bugfix/ branch → no-op error.
	// --bugfix is a cobra flag, not a positional arg; RunE receives one arg ("retype-same"),
	// so newTask == task and the no-op guard fires.
	repo := setupInitializedRepo(t)
	rimbaSuccess(t, repo, "add", "--bugfix", "retype-same")

	r := rimbaFail(t, repo, "rename", "retype-same", "--bugfix")
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

	// New branch and directory exist
	assertFileExists(t, resolver.WorktreePath(wtDir, "bugfix/retype-auth"))

	branches := testutil.GitCmd(t, repo, "branch", flagBranchList)
	if strings.Contains(branches, "feature/retype-auth") {
		t.Errorf("expected feature/retype-auth to be gone")
	}
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

// renameSetupWithRemote mirrors syncSetupWithRemote's bare-repo fixture pattern,
// minus the "commit on main" step, which --push doesn't need.
// Returns (repo path, bare remote path).
func renameSetupWithRemote(t *testing.T, task string) (string, string) {
	t.Helper()

	dir := t.TempDir()

	bareDir := filepath.Join(dir, "origin.git")
	if err := os.MkdirAll(bareDir, 0o755); err != nil {
		t.Fatal(err)
	}
	gitBare(t, bareDir, "init", "--bare", "-b", "main")

	repo := filepath.Join(dir, "repo")
	cmd := exec.Command("git", "clone", bareDir, repo)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("clone: %s: %v", out, err)
	}
	testutil.GitCmd(t, repo, "config", "user.email", "test@test.com")
	testutil.GitCmd(t, repo, "config", "user.name", "Test")

	testutil.CreateFile(t, repo, "README.md", "# Test\n")
	testutil.GitCmd(t, repo, "add", ".")
	testutil.GitCmd(t, repo, "commit", "-m", "initial commit")
	testutil.GitCmd(t, repo, "push", "-u", "origin", "main")

	rimbaSuccess(t, repo, "init")
	testutil.GitCmd(t, repo, "add", ".")
	testutil.GitCmd(t, repo, "commit", "-m", "rimba init")
	testutil.GitCmd(t, repo, "push")

	rimbaSuccess(t, repo, "add", task)

	cfg := loadConfig(t, repo)
	wtDir := filepath.Join(repo, cfg.WorktreeDir)
	branch := resolver.BranchName(defaultPrefix, task)
	wtPath := resolver.WorktreePath(wtDir, branch)

	// Give --push an existing remote branch to publish over and delete.
	testutil.GitCmd(t, wtPath, "push", "-u", "origin", branch)

	return repo, bareDir
}

// remoteBranchNames returns the set of branch names present in the bare repo.
func remoteBranchNames(t *testing.T, bareDir string) map[string]bool {
	t.Helper()
	out := gitBare(t, bareDir, "for-each-ref", "--format=%(refname:short)", "refs/heads/")
	names := make(map[string]bool)
	for line := range strings.SplitSeq(strings.TrimSpace(out), "\n") {
		if line != "" {
			names[line] = true
		}
	}
	return names
}

func TestRenamePush(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo, bareDir := renameSetupWithRemote(t, taskRenamePush)
	oldBranch := resolver.BranchName(defaultPrefix, taskRenamePush)
	newBranch := resolver.BranchName(defaultPrefix, taskRenamePush2)

	r := rimbaSuccess(t, repo, "rename", taskRenamePush, taskRenamePush2, "--push")
	assertContains(t, r.Stdout, "Published branch: origin/"+newBranch)
	assertContains(t, r.Stdout, "Deleted remote branch: origin/"+oldBranch)

	remoteBranches := remoteBranchNames(t, bareDir)
	if !remoteBranches[newBranch] {
		t.Errorf("expected origin to have branch %q, got: %v", newBranch, remoteBranches)
	}
	if remoteBranches[oldBranch] {
		t.Errorf("expected origin to no longer have branch %q", oldBranch)
	}
}

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
	taskSync      = "sync-task"
	taskSyncAll1  = "sync-all-1"
	taskSyncAll2  = "sync-all-2"
	taskSyncCflct = "sync-conflict"
)

// syncSetup creates a clean repo, adds a worktree, then makes a commit on main.
// Returns (repo, worktreePath).
func syncSetup(t *testing.T, task string) (string, string) {
	t.Helper()
	repo := setupCleanInitializedRepo(t)
	rimbaSuccess(t, repo, "add", task)

	cfg := loadConfig(t, repo)
	wtDir := filepath.Join(repo, cfg.WorktreeDir)
	branch := resolver.BranchName(defaultPrefix, task)
	wtPath := resolver.WorktreePath(wtDir, branch)

	// Make a commit on main so there's something to sync
	commitOnMain(t, repo)

	return repo, wtPath
}

// commitOnMain creates a file and commits it on main.
func commitOnMain(t *testing.T, repo string) {
	t.Helper()
	testutil.CreateFile(t, repo, fileFromMain, contentMainChg)
	testutil.GitCmd(t, repo, "add", ".")
	testutil.GitCmd(t, repo, "commit", "-m", commitUpdateMsg)
}

func TestSyncSingleWorktree(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo, wtPath := syncSetup(t, taskSync)

	r := rimbaSuccess(t, repo, "sync", taskSync)
	assertContains(t, r.Stdout, "Rebased")
	assertContains(t, r.Stdout, defaultPrefix+taskSync)

	// Verify worktree has the main change
	assertFileExists(t, filepath.Join(wtPath, fileFromMain))
}

func TestSyncSingleWithMerge(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo, wtPath := syncSetup(t, "sync-merge")

	r := rimbaSuccess(t, repo, "sync", "sync-merge", "--merge")
	assertContains(t, r.Stdout, "Merged")

	// Verify worktree has the main change
	assertFileExists(t, filepath.Join(wtPath, fileFromMain))
}

func TestSyncAll(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupCleanInitializedRepo(t)
	rimbaSuccess(t, repo, "add", taskSyncAll1)
	rimbaSuccess(t, repo, "add", taskSyncAll2)

	commitOnMain(t, repo)

	r := rimbaSuccess(t, repo, "sync", "--all")
	assertContains(t, r.Stdout, "Rebased 2 worktree(s)")

	// Verify both worktrees have the main change
	cfg := loadConfig(t, repo)
	wtDir := filepath.Join(repo, cfg.WorktreeDir)

	wt1 := resolver.WorktreePath(wtDir, resolver.BranchName(defaultPrefix, taskSyncAll1))
	wt2 := resolver.WorktreePath(wtDir, resolver.BranchName(defaultPrefix, taskSyncAll2))
	assertFileExists(t, filepath.Join(wt1, fileFromMain))
	assertFileExists(t, filepath.Join(wt2, fileFromMain))
}

func TestSyncSkipsDirty(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo, wtPath := syncSetup(t, "sync-dirty")

	// Make worktree dirty
	testutil.CreateFile(t, wtPath, "uncommitted.txt", "dirty")

	r := rimbaFail(t, repo, "sync", "sync-dirty")
	assertContains(t, r.Stderr, "uncommitted changes")
	assertContains(t, r.Stderr, "Commit or stash")
}

func TestSyncAllSkipsDirty(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupCleanInitializedRepo(t)
	rimbaSuccess(t, repo, "add", "sync-clean")
	rimbaSuccess(t, repo, "add", "sync-dirty-all")

	cfg := loadConfig(t, repo)
	wtDir := filepath.Join(repo, cfg.WorktreeDir)

	// Make one worktree dirty
	dirtyBranch := resolver.BranchName(defaultPrefix, "sync-dirty-all")
	dirtyPath := resolver.WorktreePath(wtDir, dirtyBranch)
	testutil.CreateFile(t, dirtyPath, "uncommitted.txt", "dirty")

	commitOnMain(t, repo)

	r := rimbaSuccess(t, repo, "sync", "--all")
	assertContains(t, r.Stdout, "1 skipped (dirty)")
	assertContains(t, r.Stdout, "Skipping")

	// Clean worktree should still be synced
	cleanBranch := resolver.BranchName(defaultPrefix, "sync-clean")
	cleanPath := resolver.WorktreePath(wtDir, cleanBranch)
	assertFileExists(t, filepath.Join(cleanPath, fileFromMain))
}

func TestSyncConflictAborts(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupCleanInitializedRepo(t)
	rimbaSuccess(t, repo, "add", taskSyncCflct)

	cfg := loadConfig(t, repo)
	wtDir := filepath.Join(repo, cfg.WorktreeDir)
	branch := resolver.BranchName(defaultPrefix, taskSyncCflct)
	wtPath := resolver.WorktreePath(wtDir, branch)

	// Create conflicting changes: same file modified in both
	testutil.CreateFile(t, wtPath, "conflict.txt", "worktree version")
	testutil.GitCmd(t, wtPath, "add", ".")
	testutil.GitCmd(t, wtPath, "commit", "-m", "worktree change")

	testutil.CreateFile(t, repo, "conflict.txt", "main version")
	testutil.GitCmd(t, repo, "add", ".")
	testutil.GitCmd(t, repo, "commit", "-m", contentMainChg)

	r := rimbaFail(t, repo, "sync", taskSyncCflct)
	assertContains(t, r.Stderr, "rebase")
}

func TestSyncSkipsInherited(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupCleanInitializedRepo(t)
	rimbaSuccess(t, repo, "add", "base-task")
	rimbaSuccess(t, repo, "duplicate", "base-task") // creates base-task-1

	commitOnMain(t, repo)

	r := rimbaSuccess(t, repo, "sync", "--all")
	// Only base-task should be synced, base-task-1 is inherited
	assertContains(t, r.Stdout, "Rebased 1 worktree(s)")
}

func TestSyncIncludeInherited(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupCleanInitializedRepo(t)
	rimbaSuccess(t, repo, "add", "inc-task")
	rimbaSuccess(t, repo, "duplicate", "inc-task") // creates inc-task-1

	commitOnMain(t, repo)

	r := rimbaSuccess(t, repo, "sync", "--all", "--include-inherited")
	// Both inc-task and inc-task-1 should be synced
	assertContains(t, r.Stdout, "Rebased 2 worktree(s)")
}

func TestSyncNotFound(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupInitializedRepo(t)
	r := rimbaFail(t, repo, "sync", "ghost-task")
	assertContains(t, r.Stderr, "not found")
}

func TestSyncAlreadyUpToDate(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupCleanInitializedRepo(t)
	rimbaSuccess(t, repo, "add", "uptodate-task")

	// No new commits on main — sync should succeed as no-op
	r := rimbaSuccess(t, repo, "sync", "uptodate-task")
	assertContains(t, r.Stdout, "Rebased")
}

func TestSyncNoArgsNoAll(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupInitializedRepo(t)
	r := rimbaFail(t, repo, "sync")
	assertContains(t, r.Stderr, "provide a task name or use --all")
}

// syncSetupWithRemote creates a test repo backed by a bare remote, initialises rimba,
// adds a worktree, pushes its branch to set up upstream tracking, then commits on main
// (via the bare repo round-trip) so there's something to sync.
// Returns (repo path, worktree path, bare remote path).
func syncSetupWithRemote(t *testing.T, task string) (string, string, string) {
	t.Helper()

	dir := t.TempDir()

	// 1. Create a bare repo as fake origin.
	bareDir := filepath.Join(dir, "origin.git")
	if err := os.MkdirAll(bareDir, 0o755); err != nil {
		t.Fatal(err)
	}
	gitBare(t, bareDir, "init", "--bare", "-b", "main")

	// 2. Clone the bare repo to create the working repo.
	repo := filepath.Join(dir, "repo")
	cmd := exec.Command("git", "clone", bareDir, repo)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("clone: %s: %v", out, err)
	}
	testutil.GitCmd(t, repo, "config", "user.email", "test@test.com")
	testutil.GitCmd(t, repo, "config", "user.name", "Test")

	// Create initial commit and push to origin.
	testutil.CreateFile(t, repo, "README.md", "# Test\n")
	testutil.GitCmd(t, repo, "add", ".")
	testutil.GitCmd(t, repo, "commit", "-m", "initial commit")
	testutil.GitCmd(t, repo, "push", "-u", "origin", "main")

	// 3. rimba init + commit artifacts
	rimbaSuccess(t, repo, "init")
	testutil.GitCmd(t, repo, "add", ".")
	testutil.GitCmd(t, repo, "commit", "-m", "rimba init")
	testutil.GitCmd(t, repo, "push")

	// 4. rimba add <task>
	rimbaSuccess(t, repo, "add", task)

	cfg := loadConfig(t, repo)
	wtDir := filepath.Join(repo, cfg.WorktreeDir)
	branch := resolver.BranchName(defaultPrefix, task)
	wtPath := resolver.WorktreePath(wtDir, branch)

	// 5. Push the worktree branch to set upstream tracking.
	testutil.GitCmd(t, wtPath, "push", "-u", "origin", branch)

	// 6. Commit on main so there's something to sync.
	testutil.CreateFile(t, repo, fileFromMain, contentMainChg)
	testutil.GitCmd(t, repo, "add", ".")
	testutil.GitCmd(t, repo, "commit", "-m", commitUpdateMsg)
	testutil.GitCmd(t, repo, "push")

	return repo, wtPath, bareDir
}

// gitBare runs a git command in the bare repository.
func gitBare(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v in %s: %s: %v", args, dir, out, err)
	}
	return string(out)
}

// bareRepoHEAD returns the HEAD commit SHA of a branch in the bare repo.
func bareRepoHEAD(t *testing.T, bareDir, branch string) string {
	t.Helper()
	return strings.TrimSpace(gitBare(t, bareDir, "rev-parse", branch))
}

func TestSyncPushDefault(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo, wtPath, bareDir := syncSetupWithRemote(t, "push-default")
	branch := resolver.BranchName(defaultPrefix, "push-default")

	// Get worktree HEAD before sync
	beforeSHA := strings.TrimSpace(testutil.GitCmd(t, wtPath, "rev-parse", "HEAD"))

	r := rimbaSuccess(t, repo, "sync", "push-default")
	assertContains(t, r.Stdout, "Rebased")
	assertContains(t, r.Stdout, "Pushed")

	// Verify remote branch was updated (different from before sync)
	remoteSHA := bareRepoHEAD(t, bareDir, branch)
	if remoteSHA == beforeSHA {
		t.Error("expected remote branch to be updated after sync+push")
	}

	// Verify worktree HEAD matches remote
	localSHA := strings.TrimSpace(testutil.GitCmd(t, wtPath, "rev-parse", "HEAD"))
	if remoteSHA != localSHA {
		t.Errorf("remote SHA %q != local SHA %q", remoteSHA, localSHA)
	}
}

func TestSyncPushDefaultMerge(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo, wtPath, bareDir := syncSetupWithRemote(t, "push-merge")
	branch := resolver.BranchName(defaultPrefix, "push-merge")

	r := rimbaSuccess(t, repo, "sync", "push-merge", "--merge")
	assertContains(t, r.Stdout, "Merged")
	assertContains(t, r.Stdout, "Pushed")

	// Verify remote branch was updated
	remoteSHA := bareRepoHEAD(t, bareDir, branch)
	localSHA := strings.TrimSpace(testutil.GitCmd(t, wtPath, "rev-parse", "HEAD"))
	if remoteSHA != localSHA {
		t.Errorf("remote SHA %q != local SHA %q", remoteSHA, localSHA)
	}
}

func TestSyncNoPush(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo, _, bareDir := syncSetupWithRemote(t, "no-push")
	branch := resolver.BranchName(defaultPrefix, "no-push")

	// Record remote SHA before sync
	beforeSHA := bareRepoHEAD(t, bareDir, branch)

	r := rimbaSuccess(t, repo, "sync", "no-push", "--no-push")
	assertContains(t, r.Stdout, "Rebased")
	assertNotContains(t, r.Stdout, "Pushed")

	// Verify remote branch was NOT updated
	afterSHA := bareRepoHEAD(t, bareDir, branch)
	if afterSHA != beforeSHA {
		t.Errorf("expected remote unchanged, but SHA changed from %q to %q", beforeSHA, afterSHA)
	}
}

func TestSyncPushNoUpstream(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	// Use syncSetup which has no remote — push should be silently skipped
	repo, _ := syncSetup(t, "push-noup")

	r := rimbaSuccess(t, repo, "sync", "push-noup")
	assertContains(t, r.Stdout, "Rebased")
	// "Pushed <branch> to origin" should NOT appear since there's no upstream
	assertNotContains(t, r.Stdout, "Pushed "+defaultPrefix+"push-noup")
	// No push error should appear
	assertNotContains(t, r.Stderr, "push failed")
}

func TestSyncAllPushDefault(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	dir := t.TempDir()

	// Create bare repo
	bareDir := filepath.Join(dir, "origin.git")
	if err := os.MkdirAll(bareDir, 0o755); err != nil {
		t.Fatal(err)
	}
	gitBare(t, bareDir, "init", "--bare", "-b", "main")

	// Clone
	repo := filepath.Join(dir, "repo")
	cloneCmd := exec.Command("git", "clone", bareDir, repo)
	if out, err := cloneCmd.CombinedOutput(); err != nil {
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

	// Add two worktrees with upstream
	rimbaSuccess(t, repo, "add", "sync-push-1")
	rimbaSuccess(t, repo, "add", "sync-push-2")

	cfg := loadConfig(t, repo)
	wtDir := filepath.Join(repo, cfg.WorktreeDir)

	branch1 := resolver.BranchName(defaultPrefix, "sync-push-1")
	branch2 := resolver.BranchName(defaultPrefix, "sync-push-2")
	wt1 := resolver.WorktreePath(wtDir, branch1)
	wt2 := resolver.WorktreePath(wtDir, branch2)

	testutil.GitCmd(t, wt1, "push", "-u", "origin", branch1)
	testutil.GitCmd(t, wt2, "push", "-u", "origin", branch2)

	// Commit on main
	testutil.CreateFile(t, repo, fileFromMain, contentMainChg)
	testutil.GitCmd(t, repo, "add", ".")
	testutil.GitCmd(t, repo, "commit", "-m", commitUpdateMsg)
	testutil.GitCmd(t, repo, "push")

	r := rimbaSuccess(t, repo, "sync", "--all")
	assertContains(t, r.Stdout, "2 pushed")
}

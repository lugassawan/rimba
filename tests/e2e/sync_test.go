package e2e_test

import (
	"path/filepath"
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

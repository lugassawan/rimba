package e2e_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/lugassawan/rimba/internal/resolver"
	"github.com/lugassawan/rimba/testutil"
)

const taskArchive = "archive-task"

func TestArchiveBasic(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupInitializedRepo(t)
	rimbaSuccess(t, repo, "add", taskArchive)

	cfg := loadConfig(t, repo)
	wtDir := filepath.Join(repo, cfg.WorktreeDir)
	branch := resolver.BranchName(defaultPrefix, taskArchive)
	wtPath := resolver.WorktreePath(wtDir, branch)
	assertFileExists(t, wtPath)

	r := rimbaSuccess(t, repo, "archive", taskArchive)
	assertContains(t, r.Stdout, "Archived worktree")
	assertContains(t, r.Stdout, "Branch preserved")
	assertContains(t, r.Stdout, "rimba restore")

	// Worktree directory should be gone
	assertFileNotExists(t, wtPath)

	// Branch should still exist
	out := testutil.GitCmd(t, repo, "branch", flagBranchList)
	if !strings.Contains(out, branch) {
		t.Error("expected branch to be preserved after archive")
	}
}

func TestArchiveForceDirty(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupInitializedRepo(t)
	rimbaSuccess(t, repo, "add", "dirty-archive")

	cfg := loadConfig(t, repo)
	wtDir := filepath.Join(repo, cfg.WorktreeDir)
	branch := resolver.BranchName(defaultPrefix, "dirty-archive")
	wtPath := resolver.WorktreePath(wtDir, branch)

	// Make worktree dirty
	testutil.CreateFile(t, wtPath, "dirty.txt", "uncommitted")

	// Should succeed with --force
	r := rimbaSuccess(t, repo, "archive", "dirty-archive", flagForceE2E)
	assertContains(t, r.Stdout, "Archived worktree")
	assertFileNotExists(t, wtPath)
}

func TestArchiveDirtyFails(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupInitializedRepo(t)
	rimbaSuccess(t, repo, "add", "dirty-fail")

	cfg := loadConfig(t, repo)
	wtDir := filepath.Join(repo, cfg.WorktreeDir)
	branch := resolver.BranchName(defaultPrefix, "dirty-fail")
	wtPath := resolver.WorktreePath(wtDir, branch)

	// Make worktree dirty
	testutil.CreateFile(t, wtPath, "dirty.txt", "uncommitted")
	testutil.GitCmd(t, wtPath, "add", ".")

	// Should fail without --force
	rimbaFail(t, repo, "archive", "dirty-fail")
}

func TestArchiveRestoreRoundTrip(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupInitializedRepo(t)
	rimbaSuccess(t, repo, "add", "roundtrip")

	cfg := loadConfig(t, repo)
	wtDir := filepath.Join(repo, cfg.WorktreeDir)
	branch := resolver.BranchName(defaultPrefix, "roundtrip")
	wtPath := resolver.WorktreePath(wtDir, branch)

	// Make a commit in the worktree
	testutil.CreateFile(t, wtPath, "work.txt", "important work")
	testutil.GitCmd(t, wtPath, "add", ".")
	testutil.GitCmd(t, wtPath, "commit", "-m", "add work")

	// Archive
	rimbaSuccess(t, repo, "archive", "roundtrip")
	assertFileNotExists(t, wtPath)

	// Restore
	r := rimbaSuccess(t, repo, "restore", "roundtrip", flagSkipDepsE2E, flagSkipHooksE2E)
	assertContains(t, r.Stdout, "Restored worktree")
	assertFileExists(t, wtPath)

	// Verify the committed file is present
	assertFileExists(t, filepath.Join(wtPath, "work.txt"))
}

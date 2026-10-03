package e2e_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lugassawan/rimba/internal/resolver"
	"github.com/lugassawan/rimba/testutil"
)

func TestRestoreBasic(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupInitializedRepo(t)
	rimbaSuccess(t, repo, "add", "restore-basic")

	// Archive first
	rimbaSuccess(t, repo, "archive", "restore-basic")

	// Restore
	r := rimbaSuccess(t, repo, "restore", "restore-basic", flagSkipDepsE2E, flagSkipHooksE2E)
	assertContains(t, r.Stdout, "Restored worktree")
	assertContains(t, r.Stdout, "restore-basic")
}

// A failed restore rolls back the worktree while preserving the archived branch.
func TestRestorePartialFailRollsBackKeepsBranch(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}
	if os.Getuid() == 0 {
		t.Skip("chmod 000 is ineffective for root")
	}

	repo := setupInitializedRepo(t)
	const task = "restore-hint"
	rimbaSuccess(t, repo, "add", task)
	rimbaSuccess(t, repo, "archive", task)

	envPath := filepath.Join(repo, ".env")
	if err := os.WriteFile(envPath, []byte("SECRET=fail"), 0o000); err != nil {
		t.Fatalf("write: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(envPath, 0o644) })

	cfg := loadConfig(t, repo)
	cfg.CopyFiles = []string{".env"}
	saveConfig(t, repo, cfg)
	wtDir := filepath.Join(repo, cfg.WorktreeDir)
	branch := resolver.BranchName(defaultPrefix, task)
	wtPath := resolver.WorktreePath(wtDir, branch)

	r := rimbaFail(t, repo, "restore", task)
	assertContains(t, r.Stderr, "failed to copy files")
	assertContains(t, r.Stderr, "Rollback completed")
	assertContains(t, r.Stderr, "preserved")
	assertNotContains(t, r.Stderr, "rimba remove")
	assertNotContains(t, r.Stderr, "To fix:")
	assertFileNotExists(t, wtPath)
	if got := strings.TrimSpace(testutil.GitCmd(t, repo, "branch", "--list", branch)); got == "" {
		t.Fatal("archived branch must survive restore rollback")
	}

	// With the blocker gone, restoring the same task must succeed.
	if err := os.Chmod(envPath, 0o644); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	rimbaSuccess(t, repo, "restore", task, flagSkipDepsE2E, flagSkipHooksE2E)
	assertFileExists(t, wtPath)
}

func TestRestoreNoBranch(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupInitializedRepo(t)

	// Try to restore a non-existent task
	r := rimbaFail(t, repo, "restore", "nonexistent")
	assertContains(t, r.Stderr, "no archived branch found")
}

func TestListArchivedE2E(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupInitializedRepo(t)
	rimbaSuccess(t, repo, "add", "list-archived")

	// Archive
	rimbaSuccess(t, repo, "archive", "list-archived")

	// List archived
	r := rimbaSuccess(t, repo, "list", "--archived")
	assertContains(t, r.Stdout, "Archived branches")
	assertContains(t, r.Stdout, "list-archived")
	assertContains(t, r.Stdout, "rimba restore")
}

func TestListArchivedEmpty(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupInitializedRepo(t)

	r := rimbaSuccess(t, repo, "list", "--archived")
	assertContains(t, r.Stdout, "No archived branches found")
}

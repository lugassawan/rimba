package e2e_test

import (
	"path/filepath"
	"testing"

	"github.com/lugassawan/rimba/internal/resolver"
	"github.com/lugassawan/rimba/testutil"
)

// conflictSetup creates a worktree with a committed file, ready for conflict checking.
func conflictSetup(t *testing.T, repo, task, fileName, content string) {
	t.Helper()
	rimbaSuccess(t, repo, "add", task)

	cfg := loadConfig(t, repo)
	wtDir := filepath.Join(repo, cfg.WorktreeDir)
	branch := resolver.BranchName(defaultPrefix, task)
	wtPath := resolver.WorktreePath(wtDir, branch)

	testutil.CreateFile(t, wtPath, fileName, content)
	testutil.GitCmd(t, wtPath, "add", ".")
	testutil.GitCmd(t, wtPath, "commit", "-m", "add "+fileName)
}

func TestConflictCheckNoWorktrees(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupCleanInitializedRepo(t)
	r := rimbaSuccess(t, repo, "conflict-check")
	assertContains(t, r.Stdout, "No active worktree branches")
}

func TestConflictCheckNoOverlaps(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupCleanInitializedRepo(t)
	conflictSetup(t, repo, taskConflictA, "file-a.txt", "content a")
	conflictSetup(t, repo, taskConflictB, "file-b.txt", "content b")

	r := rimbaSuccess(t, repo, "conflict-check")
	assertContains(t, r.Stdout, "No file overlaps found")
}

func TestConflictCheckWithOverlaps(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupCleanInitializedRepo(t)
	conflictSetup(t, repo, taskConflictA, "shared.txt", "content from a")
	conflictSetup(t, repo, taskConflictB, "shared.txt", "content from b")

	r := rimbaSuccess(t, repo, "conflict-check")
	assertContains(t, r.Stdout, "shared.txt")
	assertContains(t, r.Stdout, "low (2)")
	assertContains(t, r.Stdout, "1 file overlap(s)")
}

func TestConflictCheckHighSeverity(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupCleanInitializedRepo(t)
	conflictSetup(t, repo, taskConflictA, "shared.txt", "content from a")
	conflictSetup(t, repo, taskConflictB, "shared.txt", "content from b")
	conflictSetup(t, repo, taskConflictC, "shared.txt", "content from c")

	r := rimbaSuccess(t, repo, "conflict-check")
	assertContains(t, r.Stdout, "shared.txt")
	assertContains(t, r.Stdout, "high (3)")
}

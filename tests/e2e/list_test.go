package e2e_test

import (
	"path/filepath"
	"testing"

	"github.com/lugassawan/rimba/internal/resolver"
	"github.com/lugassawan/rimba/testutil"
)

func TestListShowsWorktrees(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupInitializedRepo(t)
	rimbaSuccess(t, repo, "add", taskList)

	r := rimbaSuccess(t, repo, "list")
	assertContains(t, r.Stdout, taskList)
	assertContains(t, r.Stdout, defaultPrefix+taskList)
	assertContains(t, r.Stdout, "TASK")
}

func TestListMultipleWorktrees(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupInitializedRepo(t)
	rimbaSuccess(t, repo, "add", "task-a")
	rimbaSuccess(t, repo, "add", "task-b")

	r := rimbaSuccess(t, repo, "list")
	assertContains(t, r.Stdout, "task-a")
	assertContains(t, r.Stdout, "task-b")
}

func TestListDirtyStatus(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupInitializedRepo(t)
	rimbaSuccess(t, repo, "add", "dirty-task")

	// Create an untracked file in the worktree to make it dirty
	cfg := loadConfig(t, repo)
	wtDir := filepath.Join(repo, cfg.WorktreeDir)
	branch := resolver.BranchName(defaultPrefix, "dirty-task")
	wtPath := resolver.WorktreePath(wtDir, branch)
	testutil.CreateFile(t, wtPath, "untracked.txt", "dirty")

	r := rimbaSuccess(t, repo, "list")
	assertContains(t, r.Stdout, "[dirty]")
}

func TestListFailsWithoutInit(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupRepo(t)
	rimbaFail(t, repo, "list")
}

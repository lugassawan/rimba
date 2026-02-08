package e2e_test

import (
	"path/filepath"
	"strings"
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
	assertContains(t, r.Stdout, "TYPE")
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

func TestListMultiplePrefixTypes(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupInitializedRepo(t)
	rimbaSuccess(t, repo, "add", "my-feature")
	rimbaSuccess(t, repo, "add", "--bugfix", "my-bugfix")

	r := rimbaSuccess(t, repo, "list")
	assertContains(t, r.Stdout, "feature")
	assertContains(t, r.Stdout, "bugfix")
	assertContains(t, r.Stdout, "my-feature")
	assertContains(t, r.Stdout, "my-bugfix")
}

func TestListSortedByTask(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupInitializedRepo(t)
	rimbaSuccess(t, repo, "add", "z-task")
	rimbaSuccess(t, repo, "add", "a-task")
	rimbaSuccess(t, repo, "add", "m-task")

	r := rimbaSuccess(t, repo, "list")

	// Find the positions of the task names in the output
	lines := strings.Split(r.Stdout, "\n")
	var taskOrder []string
	for _, line := range lines {
		for _, task := range []string{"a-task", "m-task", "z-task"} {
			if strings.Contains(line, task) {
				taskOrder = append(taskOrder, task)
			}
		}
	}

	if len(taskOrder) != 3 {
		t.Fatalf("expected 3 tasks in output, got %d: %v", len(taskOrder), taskOrder)
	}
	if taskOrder[0] != "a-task" || taskOrder[1] != "m-task" || taskOrder[2] != "z-task" {
		t.Errorf("expected tasks sorted as [a-task, m-task, z-task], got %v", taskOrder)
	}
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

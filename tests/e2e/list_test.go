package e2e_test

import (
	"os"
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

	// Sorted order expectation: a-task, m-task, z-task
	sortedTasks := []string{"a-task", "m-task", "z-task"}

	repo := setupInitializedRepo(t)
	// Add in reverse order to verify sorting
	rimbaSuccess(t, repo, "add", sortedTasks[2])
	rimbaSuccess(t, repo, "add", sortedTasks[0])
	rimbaSuccess(t, repo, "add", sortedTasks[1])

	r := rimbaSuccess(t, repo, "list")

	// Find the positions of the task names in the output
	lines := strings.Split(r.Stdout, "\n")
	var taskOrder []string
	for _, line := range lines {
		for _, task := range sortedTasks {
			if strings.Contains(line, task) {
				taskOrder = append(taskOrder, task)
			}
		}
	}

	if len(taskOrder) != len(sortedTasks) {
		t.Fatalf("expected %d tasks in output, got %d: %v", len(sortedTasks), len(taskOrder), taskOrder)
	}
	for i, task := range sortedTasks {
		if taskOrder[i] != task {
			t.Errorf("expected tasks sorted as %v, got %v", sortedTasks, taskOrder)
			break
		}
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

func TestListCleanStatus(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupInitializedRepo(t)
	rimbaSuccess(t, repo, "add", "clean-task")

	r := rimbaSuccess(t, repo, "list")
	assertContains(t, r.Stdout, "✓")
}

func TestListCurrentWorktreeIndicator(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupInitializedRepo(t)
	rimbaSuccess(t, repo, "add", "current-wt")

	// Resolve the worktree path
	cfg := loadConfig(t, repo)
	wtDir := filepath.Join(repo, cfg.WorktreeDir)
	branch := resolver.BranchName(defaultPrefix, "current-wt")
	wtPath := resolver.WorktreePath(wtDir, branch)

	// Copy .rimba.toml into the worktree so config lookup succeeds
	// when running from inside the worktree (git rev-parse --show-toplevel
	// returns the worktree path, not the main repo root).
	cfgSrc := filepath.Join(repo, configFile)
	cfgData, err := os.ReadFile(cfgSrc)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	if err := os.WriteFile(filepath.Join(wtPath, configFile), cfgData, 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	// Run list from inside the worktree directory
	r := rimbaSuccess(t, wtPath, "list")
	assertContains(t, r.Stdout, "* current-wt")
}

func TestListNoColor(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupInitializedRepo(t)
	rimbaSuccess(t, repo, "add", "nocolor-task")

	r := rimbaSuccess(t, repo, "list", "--no-color")
	// Should not contain any ANSI escape sequences
	assertNotContains(t, r.Stdout, "\033[")
	// Should still contain the task
	assertContains(t, r.Stdout, "nocolor-task")
}

func TestListFilterByType(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupInitializedRepo(t)
	rimbaSuccess(t, repo, "add", "feat-task")
	rimbaSuccess(t, repo, "add", "--bugfix", "bug-task")

	r := rimbaSuccess(t, repo, "list", "--type", "bugfix")
	assertContains(t, r.Stdout, "bug-task")
	assertNotContains(t, r.Stdout, "feat-task")
}

func TestListFilterDirty(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupInitializedRepo(t)
	rimbaSuccess(t, repo, "add", "clean-one")
	rimbaSuccess(t, repo, "add", taskDirtyOne)

	// Make dirty-one dirty
	cfg := loadConfig(t, repo)
	wtDir := filepath.Join(repo, cfg.WorktreeDir)
	branch := resolver.BranchName(defaultPrefix, taskDirtyOne)
	wtPath := resolver.WorktreePath(wtDir, branch)
	testutil.CreateFile(t, wtPath, "untracked.txt", "dirty")

	r := rimbaSuccess(t, repo, "list", "--dirty")
	assertContains(t, r.Stdout, taskDirtyOne)
	assertNotContains(t, r.Stdout, "clean-one")
}

func TestListFilterInvalidType(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupInitializedRepo(t)
	rimbaSuccess(t, repo, "add", "some-task")

	r := rimbaFail(t, repo, "list", "--type", "invalid")
	assertContains(t, r.Stderr, "invalid type")
	assertContains(t, r.Stderr, "valid types")
}

func TestListFailsWithoutInit(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupRepo(t)
	rimbaFail(t, repo, "list")
}

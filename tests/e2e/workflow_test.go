package e2e_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/lugassawan/rimba/internal/resolver"
	"github.com/lugassawan/rimba/testutil"
)

func TestWorkflowFullLifecycle(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupRepo(t)

	// Step 1: init
	r := rimbaSuccess(t, repo, "init")
	assertContains(t, r.Stdout, "Initialized rimba")

	cfg := loadConfig(t, repo)
	wtDir := filepath.Join(repo, cfg.WorktreeDir)

	// Step 2: add task-1
	r = rimbaSuccess(t, repo, "add", task1)
	assertContains(t, r.Stdout, msgCreatedWorktree)
	branch1 := resolver.BranchName(defaultPrefix, task1)
	wtPath1 := resolver.WorktreePath(wtDir, branch1)
	assertFileExists(t, wtPath1)

	// Step 3: add task-2 with custom prefix
	r = rimbaSuccess(t, repo, "add", "-p", bugfixPrefix, task2)
	assertContains(t, r.Stdout, msgCreatedWorktree)
	branch2 := resolver.BranchName(bugfixPrefix, task2)
	wtPath2 := resolver.WorktreePath(wtDir, branch2)
	assertFileExists(t, wtPath2)

	// Step 4: list — both present
	r = rimbaSuccess(t, repo, "list")
	assertContains(t, r.Stdout, task1)
	assertContains(t, r.Stdout, task2)

	// Step 5: remove task-1 with --branch
	r = rimbaSuccess(t, repo, "remove", "--branch", task1)
	assertContains(t, r.Stdout, "Removed worktree")
	assertContains(t, r.Stdout, "Deleted branch")
	assertFileNotExists(t, wtPath1)

	// Step 6: list — only task-2
	r = rimbaSuccess(t, repo, "list")
	assertNotContains(t, r.Stdout, defaultPrefix+task1)
	assertContains(t, r.Stdout, task2)

	// Step 7: remove task-2 with --branch (using full branch name)
	r = rimbaSuccess(t, repo, "remove", "--branch", bugfixPrefix+task2)
	assertContains(t, r.Stdout, "Removed worktree")
	assertContains(t, r.Stdout, "Deleted branch")
	assertFileNotExists(t, wtPath2)

	// Step 8: clean
	r = rimbaSuccess(t, repo, "clean")
	assertContains(t, r.Stdout, "Pruned stale worktree references")
}

func TestWorkflowDotfileCopying(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupRepo(t)

	// Create .env in repo root before init
	envContent := "DB_HOST=localhost\nDB_PORT=5432"
	testutil.CreateFile(t, repo, ".env", envContent)

	// init
	rimbaSuccess(t, repo, "init")

	// add
	rimbaSuccess(t, repo, "add", taskDotfile)

	// Verify .env was copied to worktree
	cfg := loadConfig(t, repo)
	wtDir := filepath.Join(repo, cfg.WorktreeDir)
	branch := resolver.BranchName(defaultPrefix, taskDotfile)
	wtPath := resolver.WorktreePath(wtDir, branch)

	copiedEnv := filepath.Join(wtPath, ".env")
	assertFileExists(t, copiedEnv)

	data, err := os.ReadFile(copiedEnv)
	if err != nil {
		t.Fatalf("failed to read copied .env: %v", err)
	}
	if string(data) != envContent {
		t.Errorf("expected .env content %q, got %q", envContent, string(data))
	}

	// remove (force needed because copied .env is untracked in worktree)
	rimbaSuccess(t, repo, "remove", "-f", taskDotfile)

	// Verify original .env is untouched
	originalEnv := filepath.Join(repo, ".env")
	assertFileExists(t, originalEnv)

	data, err = os.ReadFile(originalEnv)
	if err != nil {
		t.Fatalf("failed to read original .env: %v", err)
	}
	if string(data) != envContent {
		t.Errorf("expected original .env content %q, got %q", envContent, string(data))
	}
}

package e2e_test

import (
	"path/filepath"
	"testing"

	"github.com/lugassawan/rimba/internal/resolver"
	"github.com/lugassawan/rimba/testutil"
)

func TestLogBasic(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupInitializedRepo(t)
	rimbaSuccess(t, repo, "add", "log-task")

	// Make a commit in the worktree
	cfg := loadConfig(t, repo)
	wtDir := filepath.Join(repo, cfg.WorktreeDir)
	branch := resolver.BranchName(defaultPrefix, "log-task")
	wtPath := resolver.WorktreePath(wtDir, branch)
	testutil.CreateFile(t, wtPath, "work.txt", "some work")
	testutil.GitCmd(t, wtPath, "add", ".")
	testutil.GitCmd(t, wtPath, "commit", "-m", "add work file")

	r := rimbaSuccess(t, repo, "log")
	assertContains(t, r.Stdout, "Recent commits")
	assertContains(t, r.Stdout, "log-task")
	assertContains(t, r.Stdout, "add work file")
}

func TestLogLimit(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupInitializedRepo(t)
	rimbaSuccess(t, repo, "add", "log-limit-1")
	rimbaSuccess(t, repo, "add", "log-limit-2")

	r := rimbaSuccess(t, repo, "log", "--limit", "1")
	assertContains(t, r.Stdout, "Recent commits across 1 worktree(s)")
}

func TestLogWorksWithoutInit(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupRepo(t) // plain git repo, no rimba init
	r := rimbaSuccess(t, repo, "log")
	assertContains(t, r.Stdout, "No worktrees found")
}

func TestLogOrdering(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupInitializedRepo(t)
	rimbaSuccess(t, repo, "add", "log-older")
	rimbaSuccess(t, repo, "add", "log-newer")

	cfg := loadConfig(t, repo)
	wtDir := filepath.Join(repo, cfg.WorktreeDir)

	// Make a commit in the "older" worktree first
	olderBranch := resolver.BranchName(defaultPrefix, "log-older")
	olderPath := resolver.WorktreePath(wtDir, olderBranch)
	testutil.CreateFile(t, olderPath, "old.txt", "old")
	testutil.GitCmd(t, olderPath, "add", ".")
	testutil.GitCmd(t, olderPath, "commit", "-m", "older commit")

	// Then in the "newer" worktree
	newerBranch := resolver.BranchName(defaultPrefix, "log-newer")
	newerPath := resolver.WorktreePath(wtDir, newerBranch)
	testutil.CreateFile(t, newerPath, "new.txt", "new")
	testutil.GitCmd(t, newerPath, "add", ".")
	testutil.GitCmd(t, newerPath, "commit", "-m", "newer commit")

	r := rimbaSuccess(t, repo, "log")
	// The newer task should appear before the older one in output
	newerIdx := indexOf(r.Stdout, "log-newer")
	olderIdx := indexOf(r.Stdout, "log-older")
	if newerIdx == -1 || olderIdx == -1 {
		t.Fatalf("expected both tasks in output, got: %s", r.Stdout)
	}
	if newerIdx > olderIdx {
		t.Errorf("expected log-newer before log-older in output (most recent first)")
	}
}

func indexOf(s, substr string) int {
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}

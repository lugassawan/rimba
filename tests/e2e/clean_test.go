package e2e_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/lugassawan/rimba/internal/resolver"
)

func TestCleanPrunesStale(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupInitializedRepo(t)
	rimbaSuccess(t, repo, "add", "stale-task")

	// Manually delete the worktree directory to create a stale reference
	cfg := loadConfig(t, repo)
	wtDir := filepath.Join(repo, cfg.WorktreeDir)
	branch := resolver.BranchName(defaultPrefix, "stale-task")
	wtPath := resolver.WorktreePath(wtDir, branch)
	if err := os.RemoveAll(wtPath); err != nil {
		t.Fatalf("failed to remove worktree dir: %v", err)
	}

	rimbaSuccess(t, repo, "clean")
}

func TestCleanDryRun(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupInitializedRepo(t)

	r := rimbaSuccess(t, repo, "clean", "--dry-run")
	assertContains(t, r.Stdout, "Nothing to prune")
}

func TestCleanNothingToPrune(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupInitializedRepo(t)

	r := rimbaSuccess(t, repo, "clean")
	assertContains(t, r.Stdout, "Pruned stale worktree references")
}

func TestCleanWorksWithoutInit(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupRepo(t) // plain git repo, no rimba init
	rimbaSuccess(t, repo, "clean")
}

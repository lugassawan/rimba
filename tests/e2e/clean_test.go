package e2e_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lugassawan/rimba/internal/resolver"
	"github.com/lugassawan/rimba/testutil"
)

const taskNotMerged = "not-merged"

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

	r := rimbaSuccess(t, repo, "clean", flagDryRunE2E)
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

// cleanMergeSetup creates a worktree, makes a commit, and merges it into main
// (using git directly) so the branch shows as merged.
// Returns the worktree path.
func cleanMergeSetup(t *testing.T, repo, task string) string {
	t.Helper()
	rimbaSuccess(t, repo, "add", task)

	cfg := loadConfig(t, repo)
	wtDir := filepath.Join(repo, cfg.WorktreeDir)
	branch := resolver.BranchName(defaultPrefix, task)
	wtPath := resolver.WorktreePath(wtDir, branch)

	// Make a commit in the worktree
	testutil.CreateFile(t, wtPath, task+".txt", "content from "+task)
	testutil.GitCmd(t, wtPath, "add", ".")
	testutil.GitCmd(t, wtPath, "commit", "-m", "add "+task)

	// Merge into main (from repo root)
	testutil.GitCmd(t, repo, "merge", branch)

	return wtPath
}

func TestCleanMergedDetects(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupCleanInitializedRepo(t)
	cleanMergeSetup(t, repo, "clean-detect")

	r := rimbaSuccess(t, repo, "clean", flagMergedE2E, flagDryRunE2E)
	assertContains(t, r.Stdout, "Merged worktrees:")
	assertContains(t, r.Stdout, "clean-detect")
}

func TestCleanMergedDryRun(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupCleanInitializedRepo(t)
	wtPath := cleanMergeSetup(t, repo, "clean-dry")

	r := rimbaSuccess(t, repo, "clean", flagMergedE2E, flagDryRunE2E)
	assertContains(t, r.Stdout, "clean-dry")
	// Worktree should still exist (dry run)
	assertFileExists(t, wtPath)
}

func TestCleanMergedForce(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupCleanInitializedRepo(t)
	wtPath := cleanMergeSetup(t, repo, "clean-force")

	r := rimbaSuccess(t, repo, "clean", flagMergedE2E, flagForceE2E)
	assertContains(t, r.Stdout, msgRemovedWorktree)
	assertContains(t, r.Stdout, msgDeletedBranch)
	assertContains(t, r.Stdout, "Cleaned 1 merged worktree(s)")

	// Worktree should be gone
	assertFileNotExists(t, wtPath)

	// Branch should be gone
	out := testutil.GitCmd(t, repo, "branch", flagBranchList)
	if strings.Contains(out, defaultPrefix+"clean-force") {
		t.Error("expected branch to be deleted")
	}
}

func TestCleanMergedNoMerged(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupCleanInitializedRepo(t)
	// Add a worktree and make a commit so it diverges from main
	rimbaSuccess(t, repo, "add", "unmerged-task")
	cfg := loadConfig(t, repo)
	wtDir := filepath.Join(repo, cfg.WorktreeDir)
	branch := resolver.BranchName(defaultPrefix, "unmerged-task")
	wtPath := resolver.WorktreePath(wtDir, branch)
	testutil.CreateFile(t, wtPath, "wt-only.txt", "worktree content")
	testutil.GitCmd(t, wtPath, "add", ".")
	testutil.GitCmd(t, wtPath, "commit", "-m", "worktree-only commit")

	r := rimbaSuccess(t, repo, "clean", flagMergedE2E, flagForceE2E)
	assertContains(t, r.Stdout, "No merged worktrees found")
}

func TestCleanMergedKeepsUnmerged(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupCleanInitializedRepo(t)
	cleanMergeSetup(t, repo, "merged-one")

	// Add another worktree and make a commit so it diverges from main
	rimbaSuccess(t, repo, "add", taskNotMerged)
	cfg := loadConfig(t, repo)
	wtDir := filepath.Join(repo, cfg.WorktreeDir)
	unmergedBranch := resolver.BranchName(defaultPrefix, taskNotMerged)
	unmergedPath := resolver.WorktreePath(wtDir, unmergedBranch)
	testutil.CreateFile(t, unmergedPath, "wt-only.txt", "unmerged content")
	testutil.GitCmd(t, unmergedPath, "add", ".")
	testutil.GitCmd(t, unmergedPath, "commit", "-m", "unmerged commit")

	r := rimbaSuccess(t, repo, "clean", flagMergedE2E, flagForceE2E)
	assertContains(t, r.Stdout, "merged-one")
	assertNotContains(t, r.Stdout, taskNotMerged)

	// Unmerged worktree should still exist
	assertFileExists(t, unmergedPath)
}

func TestCleanMergedWorksWithoutInit(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupRepo(t) // plain git repo, no rimba init

	// Create a branch, commit, merge, then check
	testutil.GitCmd(t, repo, "checkout", "-b", "test-branch")
	testutil.CreateFile(t, repo, "test.txt", "test")
	testutil.GitCmd(t, repo, "add", ".")
	testutil.GitCmd(t, repo, "commit", "-m", "test commit")
	testutil.GitCmd(t, repo, "checkout", "main")
	testutil.GitCmd(t, repo, "merge", "test-branch")

	// clean --merged should work using DefaultBranch fallback
	// But there's no worktree for test-branch (it's just a branch), so no merged worktrees
	r := rimbaSuccess(t, repo, "clean", flagMergedE2E, flagForceE2E)
	assertContains(t, r.Stdout, "No merged worktrees found")
}

const flagStaleE2E = "--stale"

func TestCleanStaleDetects(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupInitializedRepo(t)
	rimbaSuccess(t, repo, "add", "stale-detect")

	// Use --stale-days 0 so all worktrees are stale
	r := rimbaSuccess(t, repo, "clean", flagStaleE2E, flagDryRunE2E, "--stale-days", "0")
	assertContains(t, r.Stdout, "Stale worktrees:")
	assertContains(t, r.Stdout, "stale-detect")
}

func TestCleanStaleDryRunE2E(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupInitializedRepo(t)
	rimbaSuccess(t, repo, "add", "stale-dry")

	cfg := loadConfig(t, repo)
	wtDir := filepath.Join(repo, cfg.WorktreeDir)
	branch := resolver.BranchName(defaultPrefix, "stale-dry")
	wtPath := resolver.WorktreePath(wtDir, branch)

	// Use --stale-days 0 with dry run
	r := rimbaSuccess(t, repo, "clean", flagStaleE2E, flagDryRunE2E, "--stale-days", "0")
	assertContains(t, r.Stdout, "stale-dry")
	// Worktree should still exist (dry run)
	assertFileExists(t, wtPath)
}

func TestCleanStaleForceE2E(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupInitializedRepo(t)
	rimbaSuccess(t, repo, "add", "stale-force")

	cfg := loadConfig(t, repo)
	wtDir := filepath.Join(repo, cfg.WorktreeDir)
	branch := resolver.BranchName(defaultPrefix, "stale-force")
	wtPath := resolver.WorktreePath(wtDir, branch)

	// Use --stale-days 0 + --force to remove immediately
	r := rimbaSuccess(t, repo, "clean", flagStaleE2E, flagForceE2E, "--stale-days", "0")
	assertContains(t, r.Stdout, "Cleaned 1 stale worktree(s)")

	assertFileNotExists(t, wtPath)
}

func TestCleanStaleMutualExclusive(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupInitializedRepo(t)

	// --merged and --stale should be mutually exclusive
	r := rimbaFail(t, repo, "clean", flagMergedE2E, flagStaleE2E)
	assertContains(t, r.Stderr, "if any flags in the group")
}

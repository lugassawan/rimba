package e2e_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lugassawan/rimba/internal/resolver"
	"github.com/lugassawan/rimba/testutil"
)

const taskNotMerged = "not-merged"

const flagStaleE2E = "--stale"

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
	assertContains(t, r.Stdout, "skipped remote-ref prune")
}

func TestCleanNothingToPrune(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupInitializedRepo(t)

	r := rimbaSuccess(t, repo, "clean")
	assertContains(t, r.Stdout, "Pruned stale worktree references")
	assertContains(t, r.Stdout, "skipped remote-ref prune")
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

// cleanSquashMergeSetup creates a worktree, makes a commit, and squash-merges it
// into main (using git merge --squash + git commit) so the branch content is in
// main but git branch --merged can't detect it.
// Returns the worktree path.
func cleanSquashMergeSetup(t *testing.T, repo, task string) string {
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

	// Squash merge into main (from repo root)
	testutil.GitCmd(t, repo, "merge", "--squash", branch)
	testutil.GitCmd(t, repo, "commit", "-m", "squash merge "+task)

	return wtPath
}

func TestCleanMergedDetectsSquash(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupCleanInitializedRepo(t)
	cleanSquashMergeSetup(t, repo, "squash-detect")

	r := rimbaSuccess(t, repo, "clean", flagMergedE2E, flagDryRunE2E)
	assertContains(t, r.Stdout, "Merged worktrees:")
	assertContains(t, r.Stdout, "squash-detect")
}

func TestCleanMergedSquashForce(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupCleanInitializedRepo(t)
	wtPath := cleanSquashMergeSetup(t, repo, "squash-force")

	r := rimbaSuccess(t, repo, "clean", flagMergedE2E, flagForceE2E)
	assertContains(t, r.Stdout, msgRemovedWorktree)
	assertContains(t, r.Stdout, msgDeletedBranch)
	assertContains(t, r.Stdout, "Cleaned 1 merged worktree(s)")

	// Worktree should be gone
	assertFileNotExists(t, wtPath)

	// Branch should be gone
	out := testutil.GitCmd(t, repo, "branch", flagBranchList)
	if strings.Contains(out, defaultPrefix+"squash-force") {
		t.Error("expected branch to be deleted")
	}
}

func TestCleanMergedKeepsUnmergedWithSquash(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupCleanInitializedRepo(t)
	cleanSquashMergeSetup(t, repo, "squash-merged")

	// Add an unmerged worktree
	rimbaSuccess(t, repo, "add", taskNotMerged)
	cfg := loadConfig(t, repo)
	wtDir := filepath.Join(repo, cfg.WorktreeDir)
	unmergedBranch := resolver.BranchName(defaultPrefix, taskNotMerged)
	unmergedPath := resolver.WorktreePath(wtDir, unmergedBranch)
	testutil.CreateFile(t, unmergedPath, "wt-only.txt", "unmerged content")
	testutil.GitCmd(t, unmergedPath, "add", ".")
	testutil.GitCmd(t, unmergedPath, "commit", "-m", "unmerged commit")

	r := rimbaSuccess(t, repo, "clean", flagMergedE2E, flagForceE2E)
	assertContains(t, r.Stdout, "squash-merged")
	assertNotContains(t, r.Stdout, taskNotMerged)

	// Unmerged worktree should still exist
	assertFileExists(t, unmergedPath)
}

func TestCleanMergedBothRegularAndSquash(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupCleanInitializedRepo(t)
	// One regular merge
	cleanMergeSetup(t, repo, "regular-merged")
	// One squash merge
	cleanSquashMergeSetup(t, repo, "squash-merged2")

	r := rimbaSuccess(t, repo, "clean", flagMergedE2E, flagDryRunE2E)
	assertContains(t, r.Stdout, "Merged worktrees:")
	assertContains(t, r.Stdout, "regular-merged")
	assertContains(t, r.Stdout, "squash-merged2")
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

func TestCleanFetchWarningOnStderr(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	// setupCleanInitializedRepo has no remote — fetch fails; clean --merged
	// continues with local state and exits 0 with "No merged worktrees found."
	repo := setupCleanInitializedRepo(t)

	r := rimbaSuccess(t, repo, "clean", flagMergedE2E)
	assertContains(t, r.Stderr, "Warning: fetch failed")
	assertNotContains(t, r.Stdout, "Warning: fetch failed")
}

// setupRepoWithBareOrigin creates a bare repo, clones it, runs rimba init,
// commits, and pushes. Returns (localRepo, bareDir).
func setupRepoWithBareOrigin(t *testing.T) (string, string) {
	t.Helper()

	dir := t.TempDir()

	bareDir := filepath.Join(dir, "origin.git")
	if err := os.MkdirAll(bareDir, 0o755); err != nil {
		t.Fatal(err)
	}
	gitBare(t, bareDir, "init", "--bare", "-b", "main")

	repo := filepath.Join(dir, "repo")
	cmd := exec.Command("git", "clone", bareDir, repo)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("clone: %s: %v", out, err)
	}
	testutil.GitCmd(t, repo, "config", "user.email", "test@test.com")
	testutil.GitCmd(t, repo, "config", "user.name", "Test")

	testutil.CreateFile(t, repo, "README.md", "# Test\n")
	testutil.GitCmd(t, repo, "add", ".")
	testutil.GitCmd(t, repo, "commit", "-m", "initial commit")
	testutil.GitCmd(t, repo, "push", "-u", "origin", "main")

	rimbaSuccess(t, repo, "init")
	testutil.GitCmd(t, repo, "add", ".")
	testutil.GitCmd(t, repo, "commit", "-m", "rimba init")
	testutil.GitCmd(t, repo, "push")

	return repo, bareDir
}

func TestCleanPrunesRemoteRefs(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo, bareDir := setupRepoWithBareOrigin(t)

	// Create and push a branch, then delete it from the bare repo so it's stale.
	testutil.GitCmd(t, repo, "checkout", "-b", "gone")
	testutil.GitCmd(t, repo, "push", "-u", "origin", "gone")
	testutil.GitCmd(t, repo, "checkout", "main")
	gitBare(t, bareDir, "branch", "-D", "gone")

	r := rimbaSuccess(t, repo, "clean")
	assertContains(t, r.Stdout, "Pruned remote-tracking refs: origin/gone")

	// Verify origin/gone is no longer listed.
	out := testutil.GitCmd(t, repo, "branch", "-r")
	if strings.Contains(out, "origin/gone") {
		t.Error("expected origin/gone to be pruned from remote-tracking refs")
	}
}

func TestCleanRemotePruneDryRun(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo, bareDir := setupRepoWithBareOrigin(t)

	testutil.GitCmd(t, repo, "checkout", "-b", "gone")
	testutil.GitCmd(t, repo, "push", "-u", "origin", "gone")
	testutil.GitCmd(t, repo, "checkout", "main")
	gitBare(t, bareDir, "branch", "-D", "gone")

	r := rimbaSuccess(t, repo, "clean", flagDryRunE2E)
	assertContains(t, r.Stdout, "Would prune remote-tracking refs: origin/gone")

	// Dry run: origin/gone should still be listed.
	out := testutil.GitCmd(t, repo, "branch", "-r")
	if !strings.Contains(out, "origin/gone") {
		t.Error("expected origin/gone to still exist after dry run")
	}
}

func TestCleanRemotePruneNothingStale(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo, _ := setupRepoWithBareOrigin(t)

	r := rimbaSuccess(t, repo, "clean")
	assertContains(t, r.Stdout, "No stale remote-tracking refs to prune.")
}

func TestCleanNoOriginSkipsRemotePrune(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupInitializedRepo(t)

	r := rimbaSuccess(t, repo, "clean")
	assertContains(t, r.Stdout, "No remotes; skipped remote-ref prune.")
}

func TestCleanPrunesNonOriginRemote(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	dir := t.TempDir()

	bareDir := filepath.Join(dir, "upstream.git")
	if err := os.MkdirAll(bareDir, 0o755); err != nil {
		t.Fatal(err)
	}
	gitBare(t, bareDir, "init", "--bare", "-b", "main")

	repo := filepath.Join(dir, "repo")
	cmd := exec.Command("git", "clone", bareDir, repo)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("clone: %s: %v", out, err)
	}

	testutil.GitCmd(t, repo, "config", "user.email", "test@test.com")
	testutil.GitCmd(t, repo, "config", "user.name", "Test")

	// Rename the remote from "origin" to "upstream"
	testutil.GitCmd(t, repo, "remote", "rename", "origin", "upstream")

	testutil.CreateFile(t, repo, "README.md", "# Test\n")
	testutil.GitCmd(t, repo, "add", ".")
	testutil.GitCmd(t, repo, "commit", "-m", "initial commit")
	testutil.GitCmd(t, repo, "push", "-u", "upstream", "main")

	rimbaSuccess(t, repo, "init")
	testutil.GitCmd(t, repo, "add", ".")
	testutil.GitCmd(t, repo, "commit", "-m", "rimba init")
	testutil.GitCmd(t, repo, "push", "upstream", "main")

	// Create a branch, push to upstream, then delete from bare so it's stale.
	testutil.GitCmd(t, repo, "checkout", "-b", "gone")
	testutil.GitCmd(t, repo, "push", "-u", "upstream", "gone")
	testutil.GitCmd(t, repo, "checkout", "main")
	gitBare(t, bareDir, "branch", "-D", "gone")

	r := rimbaSuccess(t, repo, "clean")
	assertContains(t, r.Stdout, "Pruned remote-tracking refs: upstream/gone")

	// Verify a subsequent fetch --prune emits no "[deleted]" line.
	fetchCmd := exec.Command("git", "-C", repo, "fetch", "--prune", "upstream")
	fetchOut, _ := fetchCmd.CombinedOutput()
	if strings.Contains(string(fetchOut), "[deleted]") {
		t.Errorf("expected no [deleted] after rimba clean, got: %s", fetchOut)
	}
}

// cleanMergeSetupWithRemote creates a worktree, commits, pushes branch to origin,
// merges into main, then pushes main to origin — so rimba sees the merge via origin/main.
func cleanMergeSetupWithRemote(t *testing.T, repo, task string) (wtPath, branch string) {
	t.Helper()
	rimbaSuccess(t, repo, "add", task)

	cfg := loadConfig(t, repo)
	wtDir := filepath.Join(repo, cfg.WorktreeDir)
	branch = resolver.BranchName(defaultPrefix, task)
	wtPath = resolver.WorktreePath(wtDir, branch)

	testutil.CreateFile(t, wtPath, task+".txt", "content from "+task)
	testutil.GitCmd(t, wtPath, "add", ".")
	testutil.GitCmd(t, wtPath, "commit", "-m", "add "+task)
	testutil.GitCmd(t, wtPath, "push", "-u", "origin", branch)
	testutil.GitCmd(t, repo, "merge", branch)
	testutil.GitCmd(t, repo, "push", "origin", "main")
	return wtPath, branch
}

func TestCleanMergedDeletesRemoteBranch(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo, _ := setupRepoWithBareOrigin(t)
	wtPath, branch := cleanMergeSetupWithRemote(t, repo, "remote-clean")

	r := rimbaSuccess(t, repo, "clean", flagMergedE2E, flagForceE2E)
	assertContains(t, r.Stdout, msgRemovedWorktree)
	assertContains(t, r.Stdout, msgDeletedBranch)
	assertContains(t, r.Stdout, "Deleted remote branch: origin/"+branch)
	assertFileNotExists(t, wtPath)

	// Local branch gone.
	out := testutil.GitCmd(t, repo, "branch", flagBranchList)
	if strings.Contains(out, branch) {
		t.Error("expected local branch to be deleted")
	}
	// Remote branch gone.
	out = testutil.GitCmd(t, repo, "branch", "-r")
	if strings.Contains(out, "origin/"+branch) {
		t.Error("expected remote branch to be deleted")
	}
}

func TestCleanMergedDryRunShowsRemote(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo, _ := setupRepoWithBareOrigin(t)
	_, branch := cleanMergeSetupWithRemote(t, repo, "remote-dry")

	r := rimbaSuccess(t, repo, "clean", flagMergedE2E, flagDryRunE2E)
	assertContains(t, r.Stdout, "will delete remote: origin/"+branch)

	// Remote branch must still be present (dry run).
	out := testutil.GitCmd(t, repo, "branch", "-r")
	if !strings.Contains(out, "origin/"+branch) {
		t.Error("expected remote branch to still exist after dry run")
	}
}

func TestCleanStaleKeepsRemote(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo, _ := setupRepoWithBareOrigin(t)

	rimbaSuccess(t, repo, "add", "stale-remote")
	cfg := loadConfig(t, repo)
	wtDir := filepath.Join(repo, cfg.WorktreeDir)
	branch := resolver.BranchName(defaultPrefix, "stale-remote")
	wtPath := resolver.WorktreePath(wtDir, branch)

	testutil.GitCmd(t, wtPath, "push", "-u", "origin", branch)

	r := rimbaSuccess(t, repo, "clean", flagStaleE2E, flagForceE2E, "--stale-days", "0")
	assertContains(t, r.Stdout, "Cleaned 1 stale worktree(s)")
	assertFileNotExists(t, wtPath)

	// Remote branch should remain (stale mode is local-only).
	out := testutil.GitCmd(t, repo, "branch", "-r")
	if !strings.Contains(out, "origin/"+branch) {
		t.Error("expected remote branch to remain after --stale clean")
	}
}

func TestCleanMergedIdempotent(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo, _ := setupRepoWithBareOrigin(t)
	cleanMergeSetupWithRemote(t, repo, "idempotent")

	// First run: clean the merged worktree (local + remote).
	rimbaSuccess(t, repo, "clean", flagMergedE2E, flagForceE2E)

	// Second run on already-clean state: should be a no-op with no failure messages.
	r := rimbaSuccess(t, repo, "clean", flagMergedE2E, flagForceE2E)
	assertContains(t, r.Stdout, "No merged worktrees found")
	assertNotContains(t, r.Stdout, "Failed")
	assertNotContains(t, r.Stderr, "Failed")
}

func TestCleanPrunesCustomRefspec(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo, bareDir := setupRepoWithBareOrigin(t)

	// Add a custom fetch refspec mapping refs/heads/custom/* → refs/remotes/origin/custom/*.
	testutil.GitCmd(t, repo, "config", "--add", "remote.origin.fetch", "+refs/heads/custom/*:refs/remotes/origin/custom/*")

	// Create and push a branch matching the custom refspec namespace.
	testutil.GitCmd(t, repo, "checkout", "-b", "custom/feature")
	testutil.GitCmd(t, repo, "push", "-u", "origin", "custom/feature")
	testutil.GitCmd(t, repo, "checkout", "main")

	// Fetch so the tracking ref exists under refs/remotes/origin/custom/feature.
	testutil.GitCmd(t, repo, "fetch", "origin")

	// Delete the branch from the bare remote so the tracking ref becomes stale.
	gitBare(t, bareDir, "branch", "-D", "custom/feature")

	r := rimbaSuccess(t, repo, "clean")
	assertContains(t, r.Stdout, "origin/custom/feature")

	// Verify a subsequent fetch --prune emits no "[deleted]" line.
	fetchCmd := exec.Command("git", "-C", repo, "fetch", "--prune")
	fetchOut, _ := fetchCmd.CombinedOutput()
	if strings.Contains(string(fetchOut), "[deleted]") {
		t.Errorf("expected no [deleted] after rimba clean, got: %s", fetchOut)
	}
}

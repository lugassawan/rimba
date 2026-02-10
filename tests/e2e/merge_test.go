package e2e_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/lugassawan/rimba/internal/resolver"
	"github.com/lugassawan/rimba/testutil"
)

// setupCleanInitializedRepo creates a repo with rimba init and commits
// the init artifacts so the repo root is clean for merge operations.
func setupCleanInitializedRepo(t *testing.T) string {
	t.Helper()
	repo := setupInitializedRepo(t)
	testutil.GitCmd(t, repo, "add", ".")
	testutil.GitCmd(t, repo, "commit", "-m", "rimba init")
	return repo
}

// mergeSetup creates a worktree with a committed file, ready for merging.
// Returns the worktree path and the file name that was committed.
func mergeSetup(t *testing.T, repo, task string, flags ...string) string { //nolint:unparam // flags kept for test flexibility
	t.Helper()
	args := append([]string{"add"}, flags...)
	args = append(args, task)
	rimbaSuccess(t, repo, args...)

	cfg := loadConfig(t, repo)
	wtDir := filepath.Join(repo, cfg.WorktreeDir)

	prefix := defaultPrefix
	for _, f := range flags {
		if f == "--bugfix" {
			prefix = bugfixPrefix
		}
	}
	branch := resolver.BranchName(prefix, task)
	wtPath := resolver.WorktreePath(wtDir, branch)

	fileName := task + ".txt"
	testutil.CreateFile(t, wtPath, fileName, "content from "+task)
	testutil.GitCmd(t, wtPath, "add", ".")
	testutil.GitCmd(t, wtPath, "commit", "-m", "add "+fileName)

	return wtPath
}

func TestMergeIntoMain(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupCleanInitializedRepo(t)
	mergeSetup(t, repo, taskMergeMain)

	r := rimbaSuccess(t, repo, "merge", taskMergeMain)
	assertContains(t, r.Stdout, "Merged")
	assertContains(t, r.Stdout, msgRemovedWorktree)
	assertContains(t, r.Stdout, msgDeletedBranch)

	// Verify file appears in repo root
	assertFileExists(t, filepath.Join(repo, taskMergeMain+".txt"))

	// Verify worktree is gone
	cfg := loadConfig(t, repo)
	wtDir := filepath.Join(repo, cfg.WorktreeDir)
	branch := resolver.BranchName(defaultPrefix, taskMergeMain)
	wtPath := resolver.WorktreePath(wtDir, branch)
	assertFileNotExists(t, wtPath)

	// Verify branch is deleted
	out := testutil.GitCmd(t, repo, "branch", flagBranchList)
	if strings.Contains(out, defaultPrefix+taskMergeMain) {
		t.Error("expected branch to be deleted")
	}
}

func TestMergeIntoMainKeep(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupCleanInitializedRepo(t)
	wtPath := mergeSetup(t, repo, taskMergeKeep)

	r := rimbaSuccess(t, repo, "merge", taskMergeKeep, "--keep")
	assertContains(t, r.Stdout, "Merged")
	assertNotContains(t, r.Stdout, msgRemovedWorktree)

	// Verify worktree still exists
	assertFileExists(t, wtPath)

	// Verify branch still exists
	out := testutil.GitCmd(t, repo, "branch", flagBranchList)
	if !strings.Contains(out, defaultPrefix+taskMergeKeep) {
		t.Error("expected branch to still exist")
	}
}

func TestMergeIntoWorktree(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupInitializedRepo(t)
	mergeSetup(t, repo, taskMergeSrc)
	targetPath := mergeSetup(t, repo, "merge-tgt")

	r := rimbaSuccess(t, repo, "merge", taskMergeSrc, flagInto, "merge-tgt")
	assertContains(t, r.Stdout, "Merged")
	// Source should still exist by default when merging into worktree
	assertNotContains(t, r.Stdout, msgRemovedWorktree)

	// Verify file appears in target worktree
	assertFileExists(t, filepath.Join(targetPath, taskMergeSrc+".txt"))

	// Verify source still exists
	cfg := loadConfig(t, repo)
	wtDir := filepath.Join(repo, cfg.WorktreeDir)
	srcBranch := resolver.BranchName(defaultPrefix, taskMergeSrc)
	srcPath := resolver.WorktreePath(wtDir, srcBranch)
	assertFileExists(t, srcPath)
}

func TestMergeIntoWorktreeDelete(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupInitializedRepo(t)
	mergeSetup(t, repo, taskMergeDelSrc)
	mergeSetup(t, repo, "merge-del-tgt")

	r := rimbaSuccess(t, repo, "merge", taskMergeDelSrc, flagInto, "merge-del-tgt", "--delete")
	assertContains(t, r.Stdout, "Merged")
	assertContains(t, r.Stdout, msgRemovedWorktree)
	assertContains(t, r.Stdout, msgDeletedBranch)

	// Verify source is gone
	cfg := loadConfig(t, repo)
	wtDir := filepath.Join(repo, cfg.WorktreeDir)
	srcBranch := resolver.BranchName(defaultPrefix, taskMergeDelSrc)
	srcPath := resolver.WorktreePath(wtDir, srcBranch)
	assertFileNotExists(t, srcPath)
}

func TestMergeSourceDirty(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupInitializedRepo(t)
	wtPath := mergeSetup(t, repo, "merge-dirty-src")

	// Make source dirty
	testutil.CreateFile(t, wtPath, "uncommitted.txt", "dirty")

	r := rimbaFail(t, repo, "merge", "merge-dirty-src")
	assertContains(t, r.Stderr, "uncommitted changes")
	assertContains(t, r.Stderr, "Commit or stash")
}

func TestMergeTargetDirty(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupInitializedRepo(t)
	mergeSetup(t, repo, "merge-tgt-dirty-src")
	targetPath := mergeSetup(t, repo, "merge-tgt-dirty")

	// Make target dirty
	testutil.CreateFile(t, targetPath, "uncommitted.txt", "dirty")

	r := rimbaFail(t, repo, "merge", "merge-tgt-dirty-src", flagInto, "merge-tgt-dirty")
	assertContains(t, r.Stderr, "uncommitted changes")
	assertContains(t, r.Stderr, "Commit or stash")
}

func TestMergeNoFF(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupCleanInitializedRepo(t)
	mergeSetup(t, repo, "merge-noff")

	rimbaSuccess(t, repo, "merge", "merge-noff", "--no-ff", "--keep")

	// Verify merge commit exists (--no-ff always creates one)
	log := testutil.GitCmd(t, repo, "log", "--oneline", "-1")
	if !strings.Contains(log, "Merge branch") {
		t.Errorf("expected merge commit, got: %s", log)
	}
}

func TestMergeSourceNotFound(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupInitializedRepo(t)
	r := rimbaFail(t, repo, "merge", "ghost-task")
	assertContains(t, r.Stderr, "not found")
}

func TestMergeTargetNotFound(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupInitializedRepo(t)
	mergeSetup(t, repo, "merge-no-tgt-src")

	r := rimbaFail(t, repo, "merge", "merge-no-tgt-src", flagInto, "ghost-target")
	assertContains(t, r.Stderr, "not found")
}

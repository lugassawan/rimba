package e2e_test

import (
	"path/filepath"
	"testing"

	"github.com/lugassawan/rimba/internal/resolver"
	"github.com/lugassawan/rimba/testutil"
)

func TestExecAll(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupInitializedRepo(t)
	rimbaSuccess(t, repo, "add", task1)
	rimbaSuccess(t, repo, "add", task2)

	r := rimbaSuccess(t, repo, "exec", "echo hello", "--all")
	assertContains(t, r.Stdout, task1)
	assertContains(t, r.Stdout, task2)
	assertContains(t, r.Stdout, "ok")
	assertContains(t, r.Stdout, "hello")
}

func TestExecTypeFilter(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupInitializedRepo(t)
	rimbaSuccess(t, repo, "add", task1)
	rimbaSuccess(t, repo, "add", "--bugfix", task2)

	r := rimbaSuccess(t, repo, "exec", "echo hi", "--type", "feature")
	assertContains(t, r.Stdout, task1)
	assertNotContains(t, r.Stdout, task2)
	assertContains(t, r.Stdout, "ok")
}

func TestExecDirtyFilter(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupInitializedRepo(t)
	rimbaSuccess(t, repo, "add", task1)
	rimbaSuccess(t, repo, "add", task2)

	// Make task-1 dirty
	cfg := loadConfig(t, repo)
	wtDir := filepath.Join(repo, cfg.WorktreeDir)
	branch1 := resolver.BranchName(defaultPrefix, task1)
	wt1Path := resolver.WorktreePath(wtDir, branch1)
	testutil.CreateFile(t, wt1Path, "dirty.txt", "dirty content")

	r := rimbaSuccess(t, repo, "exec", "echo found", "--all", "--dirty")
	assertContains(t, r.Stdout, task1)
	assertNotContains(t, r.Stdout, task2)
}

func TestExecFailFast(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupInitializedRepo(t)
	rimbaSuccess(t, repo, "add", task1)
	rimbaSuccess(t, repo, "add", task2)

	r := rimbaFail(t, repo, "exec", "exit 1", "--all", "--fail-fast")
	// At least one should show "exit 1" and some may show "cancelled"
	assertContains(t, r.Stdout, "exit 1")
}

func TestExecNonZeroExitPropagates(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupInitializedRepo(t)
	rimbaSuccess(t, repo, "add", task1)

	r := rimbaFail(t, repo, "exec", "exit 1", "--all")
	assertContains(t, r.Stdout, "exit 1")
}

func TestExecConcurrency(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupInitializedRepo(t)
	rimbaSuccess(t, repo, "add", task1)
	rimbaSuccess(t, repo, "add", task2)

	r := rimbaSuccess(t, repo, "exec", "echo ok", "--all", "--concurrency", "1")
	assertContains(t, r.Stdout, task1)
	assertContains(t, r.Stdout, task2)
	assertContains(t, r.Stdout, "ok")
}

func TestExecNoTargetFlag(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupInitializedRepo(t)
	r := rimbaFail(t, repo, "exec", "echo hi")
	assertContains(t, r.Stderr, "--all")
	assertContains(t, r.Stderr, "--type")
}

func TestExecInvalidType(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupInitializedRepo(t)
	rimbaSuccess(t, repo, "add", task1)

	r := rimbaFail(t, repo, "exec", "echo hi", "--type", "invalid")
	assertContains(t, r.Stderr, "invalid type")
}

func TestExecNoMatch(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupInitializedRepo(t)
	rimbaSuccess(t, repo, "add", task1) // adds as feature

	r := rimbaSuccess(t, repo, "exec", "echo hi", "--type", "bugfix")
	assertContains(t, r.Stdout, "No worktrees match")
}

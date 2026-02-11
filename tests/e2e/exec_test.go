package e2e_test

import (
	"testing"
)

func TestExecAll(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupInitializedRepo(t)
	rimbaSuccess(t, repo, "add", task1, flagSkipDepsE2E, flagSkipHooksE2E)
	rimbaSuccess(t, repo, "add", task2, flagSkipDepsE2E, flagSkipHooksE2E)

	r := rimbaSuccess(t, repo, "exec", cmdEchoHello, "--all")
	assertContains(t, r.Stdout, "hello")
	assertContains(t, r.Stdout, task1)
	assertContains(t, r.Stdout, task2)
	assertContains(t, r.Stdout, "2 passed")
}

func TestExecTypeFilter(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupInitializedRepo(t)
	rimbaSuccess(t, repo, "add", task1, flagSkipDepsE2E, flagSkipHooksE2E)
	rimbaSuccess(t, repo, "add", task2, "--bugfix", flagSkipDepsE2E, flagSkipHooksE2E)

	r := rimbaSuccess(t, repo, "exec", "echo found", "--type", "bugfix")
	assertContains(t, r.Stdout, task2)
	assertNotContains(t, r.Stdout, defaultPrefix+task1)
	assertContains(t, r.Stdout, "1 passed")
}

func TestExecFailFast(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupInitializedRepo(t)
	rimbaSuccess(t, repo, "add", task1, flagSkipDepsE2E, flagSkipHooksE2E)
	rimbaSuccess(t, repo, "add", task2, flagSkipDepsE2E, flagSkipHooksE2E)

	r := rimba(t, repo, "exec", "exit 1", "--all", "--fail-fast", "--concurrency", "1")
	// Command itself succeeds (exit 0) — it reports the sub-command failures in output
	assertContains(t, r.Stdout, "failed")
}

func TestExecNoFilter(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupInitializedRepo(t)
	r := rimbaFail(t, repo, "exec", cmdEchoHello)
	assertContains(t, r.Stderr, "provide at least one filter")
}

func TestExecNoWorktrees(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupInitializedRepo(t)
	r := rimbaSuccess(t, repo, "exec", cmdEchoHello, "--all")
	assertContains(t, r.Stdout, "No worktrees match")
}

func TestExecInvalidType(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupInitializedRepo(t)
	r := rimbaFail(t, repo, "exec", cmdEchoHello, "--type", "invalid")
	assertContains(t, r.Stderr, "invalid type")
}

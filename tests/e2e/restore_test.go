package e2e_test

import (
	"testing"
)

func TestRestoreBasic(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupInitializedRepo(t)
	rimbaSuccess(t, repo, "add", "restore-basic")

	// Archive first
	rimbaSuccess(t, repo, "archive", "restore-basic")

	// Restore
	r := rimbaSuccess(t, repo, "restore", "restore-basic", flagSkipDepsE2E, flagSkipHooksE2E)
	assertContains(t, r.Stdout, "Restored worktree")
	assertContains(t, r.Stdout, "restore-basic")
}

func TestRestoreNoBranch(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupInitializedRepo(t)

	// Try to restore a non-existent task
	r := rimbaFail(t, repo, "restore", "nonexistent")
	assertContains(t, r.Stderr, "no archived branch found")
}

func TestListArchivedE2E(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupInitializedRepo(t)
	rimbaSuccess(t, repo, "add", "list-archived")

	// Archive
	rimbaSuccess(t, repo, "archive", "list-archived")

	// List archived
	r := rimbaSuccess(t, repo, "list", "--archived")
	assertContains(t, r.Stdout, "Archived branches")
	assertContains(t, r.Stdout, "list-archived")
	assertContains(t, r.Stdout, "rimba restore")
}

func TestListArchivedEmpty(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupInitializedRepo(t)

	r := rimbaSuccess(t, repo, "list", "--archived")
	assertContains(t, r.Stdout, "No archived branches found")
}

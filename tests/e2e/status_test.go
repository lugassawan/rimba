package e2e_test

import (
	"testing"
)

func TestStatusBasic(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupInitializedRepo(t)
	rimbaSuccess(t, repo, "add", "status-task")

	r := rimbaSuccess(t, repo, "status")
	assertContains(t, r.Stdout, "Worktrees:")
	assertContains(t, r.Stdout, "status-task")
}

func TestStatusWorksWithoutInit(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupRepo(t) // plain git repo, no rimba init
	r := rimbaSuccess(t, repo, "status")
	assertContains(t, r.Stdout, "No worktrees found")
}

func TestStatusShowsAge(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupInitializedRepo(t)
	rimbaSuccess(t, repo, "add", "age-task")

	r := rimbaSuccess(t, repo, "status")
	assertContains(t, r.Stdout, "AGE")
	// Recently created worktree should show "just now" since it was just created
	assertContains(t, r.Stdout, "just now")
}

func TestStatusStaleMarker(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupInitializedRepo(t)
	rimbaSuccess(t, repo, "add", "stale-check")

	// Use --stale-days 0 so everything is considered stale
	r := rimbaSuccess(t, repo, "status", "--stale-days", "0")
	assertContains(t, r.Stdout, "stale")
}

func TestStatusNoWorktrees(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupInitializedRepo(t)

	r := rimbaSuccess(t, repo, "status")
	assertContains(t, r.Stdout, "No worktrees found")
}

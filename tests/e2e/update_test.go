package e2e_test

import (
	"testing"
)

func TestUpdateDevBuildShowsWarning(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupRepo(t)
	r := rimbaSuccess(t, repo, "update")
	assertContains(t, r.Stdout, "development build")
	assertContains(t, r.Stdout, "--force")
}

func TestUpdateWorksOutsideRepo(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	dir := t.TempDir() // not a git repo
	r := rimbaSuccess(t, dir, "update")
	// Should not fail with "not a git repository" error
	assertNotContains(t, r.Stderr, "not a git repository")
}

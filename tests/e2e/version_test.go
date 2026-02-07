package e2e_test

import (
	"testing"
)

func TestVersionPrintsInfo(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupRepo(t)
	r := rimbaSuccess(t, repo, "version")
	assertContains(t, r.Stdout, "rimba")
}

func TestVersionWorksOutsideRepo(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	dir := t.TempDir() // not a git repo
	r := rimbaSuccess(t, dir, "version")
	assertContains(t, r.Stdout, "rimba")
}

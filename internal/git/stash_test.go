package git_test

import (
	"context"
	"strings"
	"testing"

	"github.com/lugassawan/rimba/internal/git"
	"github.com/lugassawan/rimba/testutil"
)

func TestStashPushAndRef(t *testing.T) {
	if testing.Short() {
		t.Skip(skipIntegration)
	}

	repo := testutil.NewTestRepo(t)
	r := &git.ExecRunner{Dir: repo}
	testutil.CreateFile(t, repo, "dirty.txt", "dirty content")

	sha, err := git.StashPushAndRef(context.Background(), r, repo, "test stash")
	if err != nil {
		t.Fatalf("StashPushAndRef: %v", err)
	}
	if sha == "" {
		t.Error("expected non-empty SHA")
	}
	if strings.Contains(sha, "\n") {
		t.Errorf("SHA should not contain newline: %q", sha)
	}
}

func TestStashApply(t *testing.T) {
	if testing.Short() {
		t.Skip(skipIntegration)
	}

	repo := testutil.NewTestRepo(t)
	r := &git.ExecRunner{Dir: repo}
	testutil.CreateFile(t, repo, "stash-apply.txt", "content")

	sha, err := git.StashPushAndRef(context.Background(), r, repo, "apply test")
	if err != nil {
		t.Fatalf("StashPushAndRef: %v", err)
	}

	dirty, err := git.IsDirty(context.Background(), r, repo)
	if err != nil {
		t.Fatalf("IsDirty: %v", err)
	}
	if dirty {
		t.Error("repo should be clean after stash push")
	}

	if err := git.StashApply(r, repo, sha); err != nil {
		t.Fatalf("StashApply: %v", err)
	}

	dirty, err = git.IsDirty(context.Background(), r, repo)
	if err != nil {
		t.Fatalf("IsDirty after apply: %v", err)
	}
	if !dirty {
		t.Error("repo should be dirty after stash apply")
	}
}

func TestStashDrop(t *testing.T) {
	if testing.Short() {
		t.Skip(skipIntegration)
	}

	repo := testutil.NewTestRepo(t)
	r := &git.ExecRunner{Dir: repo}
	testutil.CreateFile(t, repo, "stash-drop.txt", "content")

	sha, err := git.StashPushAndRef(context.Background(), r, repo, "drop test")
	if err != nil {
		t.Fatalf("StashPushAndRef: %v", err)
	}

	if err := git.StashDrop(r, repo, sha); err != nil {
		t.Fatalf("StashDrop: %v", err)
	}

	stashList := testutil.GitCmd(t, repo, "stash", "list")
	if strings.TrimSpace(stashList) != "" {
		t.Errorf("stash list should be empty after drop, got: %s", stashList)
	}
}

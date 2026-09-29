package git_test

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/lugassawan/rimba/internal/git"
	"github.com/lugassawan/rimba/testutil"
)

func TestBranchReflogSubjects(t *testing.T) {
	if testing.Short() {
		t.Skip(skipIntegration)
	}

	repo := testutil.NewTestRepo(t)
	r := &git.ExecRunner{Dir: repo}
	ctx := context.Background()

	testutil.GitCmd(t, repo, "checkout", "-b", "feature/x")
	got, err := git.BranchReflogSubjects(ctx, r, "feature/x")
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"branch: Created from HEAD"}; !slices.Equal(got, want) {
		t.Errorf("fresh branch = %v, want %v", got, want)
	}

	testutil.CreateFile(t, repo, "x.txt", "x")
	testutil.GitCmd(t, repo, "add", ".")
	testutil.GitCmd(t, repo, "commit", "-m", "work")
	got, err = git.BranchReflogSubjects(ctx, r, "feature/x")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0] != "commit: work" {
		t.Errorf("after commit = %v, want commit first of 2", got)
	}
}

func TestBranchReflogSubjectsLeadingDash(t *testing.T) {
	if testing.Short() {
		t.Skip(skipIntegration)
	}

	repo := testutil.NewTestRepo(t)
	r := &git.ExecRunner{Dir: repo}

	head := strings.TrimSpace(testutil.GitCmd(t, repo, "rev-parse", "HEAD"))
	testutil.GitCmd(t, repo, "update-ref", "-m", "commit: dash", "refs/heads/-dash", head)

	got, err := git.BranchReflogSubjects(context.Background(), r, "-dash")
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"commit: dash"}; !slices.Equal(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestBranchReflogSubjectsEmptySubject(t *testing.T) {
	if testing.Short() {
		t.Skip(skipIntegration)
	}

	repo := testutil.NewTestRepo(t)
	r := &git.ExecRunner{Dir: repo}

	head := strings.TrimSpace(testutil.GitCmd(t, repo, "rev-parse", "HEAD"))
	testutil.GitCmd(t, repo, "update-ref", "-m", "commit: first", "refs/heads/e", head)
	testutil.CreateFile(t, repo, "n.txt", "n")
	testutil.GitCmd(t, repo, "add", ".")
	testutil.GitCmd(t, repo, "commit", "-m", "next")
	next := strings.TrimSpace(testutil.GitCmd(t, repo, "rev-parse", "HEAD"))
	testutil.GitCmd(t, repo, "update-ref", "refs/heads/e", next)

	got, err := git.BranchReflogSubjects(context.Background(), r, "e")
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"", "commit: first"}; !slices.Equal(got, want) {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestBranchReflogSubjectsNoReflog(t *testing.T) {
	if testing.Short() {
		t.Skip(skipIntegration)
	}

	repo := testutil.NewTestRepo(t)
	r := &git.ExecRunner{Dir: repo}

	head := strings.TrimSpace(testutil.GitCmd(t, repo, "rev-parse", "HEAD"))
	testutil.GitCmd(t, repo, "update-ref", "refs/heads/bare", head)
	if err := os.Remove(filepath.Join(repo, ".git", "logs", "refs", "heads", "bare")); err != nil {
		t.Fatal(err)
	}

	got, err := git.BranchReflogSubjects(context.Background(), r, "bare")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Errorf("got %v, want empty", got)
	}
}

package git_test

import (
	"path/filepath"
	"testing"

	"github.com/lugassawan/rimba/internal/git"
	"github.com/lugassawan/rimba/testutil"
)

func TestRepoRoot(t *testing.T) {
	if testing.Short() {
		t.Skip(skipIntegration)
	}

	repo := testutil.NewTestRepo(t)
	r := &git.ExecRunner{Dir: repo}

	root, err := git.RepoRoot(r)
	if err != nil {
		t.Fatalf("RepoRoot: %v", err)
	}

	// Resolve symlinks for macOS /private/var vs /var
	wantRoot, _ := filepath.EvalSymlinks(repo)
	gotRoot, _ := filepath.EvalSymlinks(root)
	if gotRoot != wantRoot {
		t.Errorf("RepoRoot = %q, want %q", gotRoot, wantRoot)
	}
}

func TestRepoName(t *testing.T) {
	if testing.Short() {
		t.Skip(skipIntegration)
	}

	repo := testutil.NewTestRepo(t)
	r := &git.ExecRunner{Dir: repo}

	name, err := git.RepoName(r)
	if err != nil {
		t.Fatalf("RepoName: %v", err)
	}

	if name != "test-repo" {
		t.Errorf("RepoName = %q, want %q", name, "test-repo")
	}
}

func TestDefaultBranch(t *testing.T) {
	if testing.Short() {
		t.Skip(skipIntegration)
	}

	repo := testutil.NewTestRepo(t)
	r := &git.ExecRunner{Dir: repo}

	branch, err := git.DefaultBranch(r)
	if err != nil {
		t.Fatalf("DefaultBranch: %v", err)
	}

	if branch != "main" {
		t.Errorf("DefaultBranch = %q, want %q", branch, "main")
	}
}

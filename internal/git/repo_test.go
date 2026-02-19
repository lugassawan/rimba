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

func TestHooksDir(t *testing.T) {
	if testing.Short() {
		t.Skip(skipIntegration)
	}

	repo := testutil.NewTestRepo(t)
	r := &git.ExecRunner{Dir: repo}

	dir, err := git.HooksDir(r)
	if err != nil {
		t.Fatalf("HooksDir: %v", err)
	}

	// Resolve symlinks for macOS /private/var vs /var
	wantDir, _ := filepath.EvalSymlinks(filepath.Join(repo, ".git", "hooks"))
	gotDir, _ := filepath.EvalSymlinks(dir)
	if gotDir != wantDir {
		t.Errorf("HooksDir = %q, want %q", gotDir, wantDir)
	}
}

func TestHooksDirWithCoreHooksPath(t *testing.T) {
	if testing.Short() {
		t.Skip(skipIntegration)
	}

	repo := testutil.NewTestRepo(t)

	// Set core.hooksPath to .githooks
	testutil.GitCmd(t, repo, "config", "core.hooksPath", ".githooks")

	r := &git.ExecRunner{Dir: repo}
	dir, err := git.HooksDir(r)
	if err != nil {
		t.Fatalf("HooksDir: %v", err)
	}

	// Resolve symlinks for macOS /private/var vs /var
	wantDir, _ := filepath.EvalSymlinks(filepath.Join(repo, ".githooks"))
	gotDir, _ := filepath.EvalSymlinks(dir)
	if gotDir != wantDir {
		t.Errorf("HooksDir = %q, want %q", gotDir, wantDir)
	}
}

func TestMainRepoRoot(t *testing.T) {
	if testing.Short() {
		t.Skip(skipIntegration)
	}

	repo := testutil.NewTestRepo(t)
	r := &git.ExecRunner{Dir: repo}

	root, err := git.MainRepoRoot(r)
	if err != nil {
		t.Fatalf("MainRepoRoot: %v", err)
	}

	// From the main repo, MainRepoRoot should equal RepoRoot
	wantRoot, _ := filepath.EvalSymlinks(repo)
	gotRoot, _ := filepath.EvalSymlinks(root)
	if gotRoot != wantRoot {
		t.Errorf("MainRepoRoot = %q, want %q", gotRoot, wantRoot)
	}
}

func TestMainRepoRootFromWorktree(t *testing.T) {
	if testing.Short() {
		t.Skip(skipIntegration)
	}

	repo := testutil.NewTestRepo(t)
	r := &git.ExecRunner{Dir: repo}

	// Create a worktree
	wtPath := filepath.Join(repo, ".worktrees", "test-wt")
	if err := git.AddWorktree(r, wtPath, "feature/test-wt", "main"); err != nil {
		t.Fatalf("AddWorktree: %v", err)
	}

	// Run MainRepoRoot from the worktree
	wr := &git.ExecRunner{Dir: wtPath}
	root, err := git.MainRepoRoot(wr)
	if err != nil {
		t.Fatalf("MainRepoRoot from worktree: %v", err)
	}

	// Should point to the main repo, not the worktree
	wantRoot, _ := filepath.EvalSymlinks(repo)
	gotRoot, _ := filepath.EvalSymlinks(root)
	if gotRoot != wantRoot {
		t.Errorf("MainRepoRoot from worktree = %q, want main repo %q", gotRoot, wantRoot)
	}

	// Confirm RepoRoot would give different result (the worktree path)
	wtRoot, err := git.RepoRoot(wr)
	if err != nil {
		t.Fatalf("RepoRoot from worktree: %v", err)
	}
	wtRootResolved, _ := filepath.EvalSymlinks(wtRoot)
	wtPathResolved, _ := filepath.EvalSymlinks(wtPath)
	if wtRootResolved != wtPathResolved {
		t.Errorf("RepoRoot from worktree = %q, want worktree path %q", wtRootResolved, wtPathResolved)
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

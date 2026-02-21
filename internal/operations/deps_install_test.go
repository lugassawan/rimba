package operations

import (
	"testing"

	"github.com/lugassawan/rimba/internal/git"
)

func TestWorktreePathsExcluding(t *testing.T) {
	entries := []git.WorktreeEntry{
		{Path: "/wt/a", Branch: "a"},
		{Path: "/wt/b", Branch: "b"},
		{Path: "/wt/c", Branch: "c"},
	}

	got := WorktreePathsExcluding(entries, "/wt/b")
	if len(got) != 2 {
		t.Fatalf("expected 2 paths, got %d", len(got))
	}
	if got[0] != "/wt/a" || got[1] != "/wt/c" {
		t.Errorf("unexpected paths: %v", got)
	}
}

func TestWorktreePathsExcluding_NoMatch(t *testing.T) {
	entries := []git.WorktreeEntry{
		{Path: "/wt/a", Branch: "a"},
	}

	got := WorktreePathsExcluding(entries, "/wt/nonexistent")
	if len(got) != 1 {
		t.Fatalf("expected 1 path, got %d", len(got))
	}
}

func TestWorktreePathsExcluding_Empty(t *testing.T) {
	got := WorktreePathsExcluding(nil, "/wt/a")
	if got != nil {
		t.Errorf("expected nil, got %v", got)
	}
}

func TestResolveMainBranch_ConfigDefault(t *testing.T) {
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			t.Fatal("git should not be called when configDefault is set")
			return "", nil
		},
	}

	branch, err := ResolveMainBranch(r, "develop")
	if err != nil {
		t.Fatal(err)
	}
	if branch != "develop" {
		t.Errorf("expected %q, got %q", "develop", branch)
	}
}

func TestResolveMainBranch_FallbackToGit(t *testing.T) {
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			// git.DefaultBranch calls: git symbolic-ref refs/remotes/origin/HEAD
			return "refs/remotes/origin/main", nil
		},
	}

	branch, err := ResolveMainBranch(r, "")
	if err != nil {
		t.Fatal(err)
	}
	if branch != "main" {
		t.Errorf("expected %q, got %q", "main", branch)
	}
}

func TestResolveMainBranch_GitError(t *testing.T) {
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			return "", errGitFailed
		},
	}

	_, err := ResolveMainBranch(r, "")
	if err == nil {
		t.Fatal("expected error")
	}
}

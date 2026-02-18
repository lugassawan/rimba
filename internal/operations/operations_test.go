package operations

import (
	"strings"
	"testing"

	"github.com/lugassawan/rimba/internal/resolver"
)

func TestListWorktreeInfos(t *testing.T) {
	porcelain := strings.Join([]string{
		"worktree /repo",
		"HEAD abc123",
		"branch refs/heads/main",
		"",
		"worktree /repo-worktrees/feature-login",
		"HEAD def456",
		"branch refs/heads/feature/login",
		"",
	}, "\n")

	r := &mockRunner{
		run:      func(_ ...string) (string, error) { return porcelain, nil },
		runInDir: noopRunInDir,
	}

	infos, err := ListWorktreeInfos(r)
	if err != nil {
		t.Fatalf("ListWorktreeInfos: %v", err)
	}
	if len(infos) != 2 {
		t.Fatalf("got %d worktrees, want 2", len(infos))
	}
	if infos[0].Branch != branchMain {
		t.Errorf("infos[0].Branch = %q, want %q", infos[0].Branch, branchMain)
	}
	if infos[1].Branch != branchFeature {
		t.Errorf("infos[1].Branch = %q, want %q", infos[1].Branch, branchFeature)
	}
}

func TestListWorktreeInfosError(t *testing.T) {
	r := &mockRunner{
		run:      func(_ ...string) (string, error) { return "", errGitFailed },
		runInDir: noopRunInDir,
	}
	if _, err := ListWorktreeInfos(r); err == nil {
		t.Fatal("expected error")
	}
}

func TestFindWorktree(t *testing.T) {
	porcelain := strings.Join([]string{
		"worktree /repo",
		"HEAD abc123",
		"branch refs/heads/main",
		"",
		"worktree /repo-worktrees/feature-login",
		"HEAD def456",
		"branch refs/heads/feature/login",
		"",
	}, "\n")

	r := &mockRunner{
		run:      func(_ ...string) (string, error) { return porcelain, nil },
		runInDir: noopRunInDir,
	}

	t.Run("found", func(t *testing.T) {
		wt, err := FindWorktree(r, "login")
		if err != nil {
			t.Fatalf("FindWorktree: %v", err)
		}
		if wt.Branch != branchFeature {
			t.Errorf("Branch = %q, want %q", wt.Branch, branchFeature)
		}
	})

	t.Run("not found", func(t *testing.T) {
		if _, err := FindWorktree(r, "nonexistent"); err == nil {
			t.Fatal("expected error for missing worktree")
		}
	})
}

func TestFindWorktreeError(t *testing.T) {
	r := &mockRunner{
		run:      func(_ ...string) (string, error) { return "", errGitFailed },
		runInDir: noopRunInDir,
	}
	if _, err := FindWorktree(r, "login"); err == nil {
		t.Fatal("expected error")
	}
}

func TestFilterByType(t *testing.T) {
	prefixes := resolver.AllPrefixes()
	worktrees := []resolver.WorktreeInfo{
		{Branch: branchFeature},
		{Branch: branchBugfixTypo},
		{Branch: branchMain},
		{Branch: "feature/signup"},
	}

	features := FilterByType(worktrees, prefixes, "feature")
	if len(features) != 2 {
		t.Fatalf("expected 2 feature worktrees, got %d", len(features))
	}
	if features[0].Branch != branchFeature {
		t.Errorf("features[0] = %q, want %q", features[0].Branch, branchFeature)
	}
	if features[1].Branch != "feature/signup" {
		t.Errorf("features[1] = %q, want %q", features[1].Branch, "feature/signup")
	}

	bugfixes := FilterByType(worktrees, prefixes, "bugfix")
	if len(bugfixes) != 1 {
		t.Fatalf("expected 1 bugfix worktree, got %d", len(bugfixes))
	}

	hotfixes := FilterByType(worktrees, prefixes, "hotfix")
	if len(hotfixes) != 0 {
		t.Errorf("expected 0 hotfix worktrees, got %d", len(hotfixes))
	}
}

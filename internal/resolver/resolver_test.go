package resolver_test

import (
	"testing"

	"github.com/lugassawan/rimba/internal/resolver"
)

func TestBranchName(t *testing.T) {
	tests := []struct {
		prefix, task, want string
	}{
		{"feat/", "my-task", "feat/my-task"},
		{"bugfix/", "login-fix", "bugfix/login-fix"},
		{"", "bare-branch", "bare-branch"},
	}
	for _, tt := range tests {
		got := resolver.BranchName(tt.prefix, tt.task)
		if got != tt.want {
			t.Errorf("BranchName(%q, %q) = %q, want %q", tt.prefix, tt.task, got, tt.want)
		}
	}
}

func TestDirName(t *testing.T) {
	tests := []struct {
		branch, want string
	}{
		{"feat/my-task", "feat-my-task"},
		{"bugfix/login-fix", "bugfix-login-fix"},
		{"bare-branch", "bare-branch"},
	}
	for _, tt := range tests {
		got := resolver.DirName(tt.branch)
		if got != tt.want {
			t.Errorf("DirName(%q) = %q, want %q", tt.branch, got, tt.want)
		}
	}
}

func TestWorktreePath(t *testing.T) {
	got := resolver.WorktreePath("/home/user/repo-worktrees", "feat/my-task")
	want := "/home/user/repo-worktrees/feat-my-task"
	if got != want {
		t.Errorf("WorktreePath = %q, want %q", got, want)
	}
}

func TestTaskFromBranch(t *testing.T) {
	tests := []struct {
		branch, prefix, want string
	}{
		{"feat/my-task", "feat/", "my-task"},
		{"bugfix/login-fix", "bugfix/", "login-fix"},
		{"feat/my-task", "bugfix/", "feat/my-task"},
		{"bare-branch", "feat/", "bare-branch"},
	}
	for _, tt := range tests {
		got := resolver.TaskFromBranch(tt.branch, tt.prefix)
		if got != tt.want {
			t.Errorf("TaskFromBranch(%q, %q) = %q, want %q", tt.branch, tt.prefix, got, tt.want)
		}
	}
}

func TestFindBranchForTask(t *testing.T) {
	worktrees := []resolver.WorktreeInfo{
		{Path: "/wt/feat-login", Branch: "feat/login"},
		{Path: "/wt/feat-signup", Branch: "feat/signup"},
		{Path: "/wt/bugfix-crash", Branch: "bugfix/crash"},
	}

	t.Run("match with prefix", func(t *testing.T) {
		wt, ok := resolver.FindBranchForTask("login", worktrees, "feat/")
		if !ok {
			t.Fatal("expected match")
		}
		if wt.Branch != "feat/login" {
			t.Errorf("got branch %q, want %q", wt.Branch, "feat/login")
		}
	})

	t.Run("match with full branch name", func(t *testing.T) {
		wt, ok := resolver.FindBranchForTask("bugfix/crash", worktrees, "feat/")
		if !ok {
			t.Fatal("expected match")
		}
		if wt.Branch != "bugfix/crash" {
			t.Errorf("got branch %q, want %q", wt.Branch, "bugfix/crash")
		}
	})

	t.Run("no match", func(t *testing.T) {
		_, ok := resolver.FindBranchForTask("nonexistent", worktrees, "feat/")
		if ok {
			t.Fatal("expected no match")
		}
	})
}

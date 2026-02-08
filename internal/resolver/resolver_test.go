package resolver_test

import (
	"testing"

	"github.com/lugassawan/rimba/internal/resolver"
)

func TestBranchName(t *testing.T) {
	tests := []struct {
		prefix, task, want string
	}{
		{"feature/", "my-task", "feature/my-task"},
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
		{"feature/my-task", "feature-my-task"},
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
	got := resolver.WorktreePath("/home/user/repo-worktrees", "feature/my-task")
	want := "/home/user/repo-worktrees/feature-my-task"
	if got != want {
		t.Errorf("WorktreePath = %q, want %q", got, want)
	}
}

func TestTaskFromBranch(t *testing.T) {
	prefixes := resolver.AllPrefixes()

	tests := []struct {
		branch     string
		wantTask   string
		wantPrefix string
	}{
		{"feature/my-task", "my-task", "feature/"},
		{"bugfix/login-fix", "login-fix", "bugfix/"},
		{"hotfix/urgent", "urgent", "hotfix/"},
		{"docs/readme", "readme", "docs/"},
		{"test/experiment", "experiment", "test/"},
		{"chore/deps", "deps", "chore/"},
		{"bare-branch", "bare-branch", ""},
		{"unknown/prefix", "unknown/prefix", ""},
	}
	for _, tt := range tests {
		task, matched := resolver.TaskFromBranch(tt.branch, prefixes)
		if task != tt.wantTask {
			t.Errorf("TaskFromBranch(%q) task = %q, want %q", tt.branch, task, tt.wantTask)
		}
		if matched != tt.wantPrefix {
			t.Errorf("TaskFromBranch(%q) prefix = %q, want %q", tt.branch, matched, tt.wantPrefix)
		}
	}
}

func TestFindBranchForTask(t *testing.T) {
	worktrees := []resolver.WorktreeInfo{
		{Path: "/wt/feature-login", Branch: "feature/login"},
		{Path: "/wt/feature-signup", Branch: "feature/signup"},
		{Path: "/wt/bugfix-crash", Branch: "bugfix/crash"},
	}

	prefixes := resolver.AllPrefixes()

	t.Run("match with prefix", func(t *testing.T) {
		wt, ok := resolver.FindBranchForTask("login", worktrees, prefixes)
		if !ok {
			t.Fatal("expected match")
		}
		if wt.Branch != "feature/login" {
			t.Errorf("got branch %q, want %q", wt.Branch, "feature/login")
		}
	})

	t.Run("cross-prefix match", func(t *testing.T) {
		wt, ok := resolver.FindBranchForTask("crash", worktrees, prefixes)
		if !ok {
			t.Fatal("expected match")
		}
		if wt.Branch != "bugfix/crash" {
			t.Errorf("got branch %q, want %q", wt.Branch, "bugfix/crash")
		}
	})

	t.Run("match with full branch name", func(t *testing.T) {
		wt, ok := resolver.FindBranchForTask("bugfix/crash", worktrees, prefixes)
		if !ok {
			t.Fatal("expected match")
		}
		if wt.Branch != "bugfix/crash" {
			t.Errorf("got branch %q, want %q", wt.Branch, "bugfix/crash")
		}
	})

	t.Run("no match", func(t *testing.T) {
		_, ok := resolver.FindBranchForTask("nonexistent", worktrees, prefixes)
		if ok {
			t.Fatal("expected no match")
		}
	})
}

func TestAllPrefixes(t *testing.T) {
	prefixes := resolver.AllPrefixes()
	if len(prefixes) != 6 {
		t.Fatalf("expected 6 prefixes, got %d", len(prefixes))
	}
	// First should always be feature/ (the default)
	if prefixes[0] != "feature/" {
		t.Errorf("expected first prefix to be %q, got %q", "feature/", prefixes[0])
	}
	// All should end with /
	for _, p := range prefixes {
		if p[len(p)-1] != '/' {
			t.Errorf("prefix %q does not end with /", p)
		}
	}
}

func TestPrefixString(t *testing.T) {
	tests := []struct {
		pt     resolver.PrefixType
		want   string
		wantOk bool
	}{
		{resolver.PrefixFeature, "feature/", true},
		{resolver.PrefixBugfix, "bugfix/", true},
		{resolver.PrefixHotfix, "hotfix/", true},
		{resolver.PrefixDocs, "docs/", true},
		{resolver.PrefixTest, "test/", true},
		{resolver.PrefixChore, "chore/", true},
		{resolver.PrefixType("unknown"), "", false},
	}
	for _, tt := range tests {
		got, ok := resolver.PrefixString(tt.pt)
		if ok != tt.wantOk || got != tt.want {
			t.Errorf("PrefixString(%q) = (%q, %v), want (%q, %v)", tt.pt, got, ok, tt.want, tt.wantOk)
		}
	}
}

func TestValidPrefixType(t *testing.T) {
	valid := []string{"feature", "bugfix", "hotfix", "docs", "test", "chore"}
	for _, v := range valid {
		if !resolver.ValidPrefixType(v) {
			t.Errorf("ValidPrefixType(%q) = false, want true", v)
		}
	}

	invalid := []string{"feat", "release", "unknown", ""}
	for _, v := range invalid {
		if resolver.ValidPrefixType(v) {
			t.Errorf("ValidPrefixType(%q) = true, want false", v)
		}
	}
}

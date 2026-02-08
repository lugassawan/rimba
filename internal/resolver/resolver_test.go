package resolver_test

import (
	"testing"

	"github.com/lugassawan/rimba/internal/resolver"
)

const (
	taskMyTask   = "my-task"
	taskLoginFix = "login-fix"
	taskBare     = "bare-branch"
	taskLogin    = "login"
	taskCrash    = "crash"

	msgExpectedMatch = "expected match"
	fmtGotBranch     = "got branch %q, want %q"
)

var (
	featurePrefix, _ = resolver.PrefixString(resolver.PrefixFeature)
	bugfixPrefix, _  = resolver.PrefixString(resolver.PrefixBugfix)
	hotfixPrefix, _  = resolver.PrefixString(resolver.PrefixHotfix)
	docsPrefix, _    = resolver.PrefixString(resolver.PrefixDocs)
	testPrefix, _    = resolver.PrefixString(resolver.PrefixTest)
	chorePrefix, _   = resolver.PrefixString(resolver.PrefixChore)
)

func TestBranchName(t *testing.T) {
	tests := []struct {
		prefix, task, want string
	}{
		{featurePrefix, taskMyTask, featurePrefix + taskMyTask},
		{bugfixPrefix, taskLoginFix, bugfixPrefix + taskLoginFix},
		{"", taskBare, taskBare},
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
		{featurePrefix + taskMyTask, "feature-my-task"},
		{bugfixPrefix + taskLoginFix, "bugfix-login-fix"},
		{taskBare, taskBare},
	}
	for _, tt := range tests {
		got := resolver.DirName(tt.branch)
		if got != tt.want {
			t.Errorf("DirName(%q) = %q, want %q", tt.branch, got, tt.want)
		}
	}
}

func TestWorktreePath(t *testing.T) {
	got := resolver.WorktreePath("/home/user/repo-worktrees", featurePrefix+taskMyTask)
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
		{featurePrefix + taskMyTask, taskMyTask, featurePrefix},
		{bugfixPrefix + taskLoginFix, taskLoginFix, bugfixPrefix},
		{hotfixPrefix + "urgent", "urgent", hotfixPrefix},
		{docsPrefix + "readme", "readme", docsPrefix},
		{testPrefix + "experiment", "experiment", testPrefix},
		{chorePrefix + "deps", "deps", chorePrefix},
		{taskBare, taskBare, ""},
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
		{Path: "/wt/feature-login", Branch: featurePrefix + taskLogin},
		{Path: "/wt/feature-signup", Branch: featurePrefix + "signup"},
		{Path: "/wt/bugfix-crash", Branch: bugfixPrefix + taskCrash},
	}

	prefixes := resolver.AllPrefixes()

	t.Run("match with prefix", func(t *testing.T) {
		wt, ok := resolver.FindBranchForTask(taskLogin, worktrees, prefixes)
		if !ok {
			t.Fatal(msgExpectedMatch)
		}
		if wt.Branch != featurePrefix+taskLogin {
			t.Errorf(fmtGotBranch, wt.Branch, featurePrefix+taskLogin)
		}
	})

	t.Run("cross-prefix match", func(t *testing.T) {
		wt, ok := resolver.FindBranchForTask(taskCrash, worktrees, prefixes)
		if !ok {
			t.Fatal(msgExpectedMatch)
		}
		if wt.Branch != bugfixPrefix+taskCrash {
			t.Errorf(fmtGotBranch, wt.Branch, bugfixPrefix+taskCrash)
		}
	})

	t.Run("match with full branch name", func(t *testing.T) {
		wt, ok := resolver.FindBranchForTask(bugfixPrefix+taskCrash, worktrees, prefixes)
		if !ok {
			t.Fatal(msgExpectedMatch)
		}
		if wt.Branch != bugfixPrefix+taskCrash {
			t.Errorf(fmtGotBranch, wt.Branch, bugfixPrefix+taskCrash)
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
	if prefixes[0] != featurePrefix {
		t.Errorf("expected first prefix to be %q, got %q", featurePrefix, prefixes[0])
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
		{resolver.PrefixFeature, featurePrefix, true},
		{resolver.PrefixBugfix, bugfixPrefix, true},
		{resolver.PrefixHotfix, hotfixPrefix, true},
		{resolver.PrefixDocs, docsPrefix, true},
		{resolver.PrefixTest, testPrefix, true},
		{resolver.PrefixChore, chorePrefix, true},
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

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

func TestPureTaskFromBranch(t *testing.T) {
	prefixes := resolver.AllPrefixes()

	const svcAuthAPI = "auth-api"

	tests := []struct {
		branch     string
		wantTask   string
		wantPrefix string
	}{
		{featurePrefix + taskMyTask, taskMyTask, featurePrefix},
		{bugfixPrefix + taskLoginFix, taskLoginFix, bugfixPrefix},
		{svcAuthAPI + "/" + featurePrefix + taskLogin, taskLogin, featurePrefix},
		{svcAuthAPI + "/" + bugfixPrefix + taskCrash, taskCrash, bugfixPrefix},
		{taskBare, taskBare, ""},
		{"unknown/prefix", "unknown/prefix", ""},
	}
	for _, tt := range tests {
		task, matched := resolver.PureTaskFromBranch(tt.branch, prefixes)
		if task != tt.wantTask {
			t.Errorf("PureTaskFromBranch(%q) task = %q, want %q", tt.branch, task, tt.wantTask)
		}
		if matched != tt.wantPrefix {
			t.Errorf("PureTaskFromBranch(%q) prefix = %q, want %q", tt.branch, matched, tt.wantPrefix)
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
		wt, ok := resolver.FindBranchForTask("", taskLogin, worktrees, prefixes)
		if !ok {
			t.Fatal(msgExpectedMatch)
		}
		if wt.Branch != featurePrefix+taskLogin {
			t.Errorf(fmtGotBranch, wt.Branch, featurePrefix+taskLogin)
		}
	})

	t.Run("cross-prefix match", func(t *testing.T) {
		wt, ok := resolver.FindBranchForTask("", taskCrash, worktrees, prefixes)
		if !ok {
			t.Fatal(msgExpectedMatch)
		}
		if wt.Branch != bugfixPrefix+taskCrash {
			t.Errorf(fmtGotBranch, wt.Branch, bugfixPrefix+taskCrash)
		}
	})

	t.Run("match with full branch name", func(t *testing.T) {
		wt, ok := resolver.FindBranchForTask("", bugfixPrefix+taskCrash, worktrees, prefixes)
		if !ok {
			t.Fatal(msgExpectedMatch)
		}
		if wt.Branch != bugfixPrefix+taskCrash {
			t.Errorf(fmtGotBranch, wt.Branch, bugfixPrefix+taskCrash)
		}
	})

	t.Run("no match", func(t *testing.T) {
		_, ok := resolver.FindBranchForTask("", "nonexistent", worktrees, prefixes)
		if ok {
			t.Fatal("expected no match")
		}
	})
}

func TestFullBranchName(t *testing.T) {
	tests := []struct {
		service, prefix, task, want string
	}{
		{"auth-api", featurePrefix, "my-task", "auth-api/feature/my-task"},
		{"auth-api", bugfixPrefix, "login-fix", "auth-api/bugfix/login-fix"},
		{"", featurePrefix, "my-task", "feature/my-task"},
		{"", "", "bare", "bare"},
	}
	for _, tt := range tests {
		got := resolver.FullBranchName(tt.service, tt.prefix, tt.task)
		if got != tt.want {
			t.Errorf("FullBranchName(%q, %q, %q) = %q, want %q", tt.service, tt.prefix, tt.task, got, tt.want)
		}
	}
}

func TestSplitServiceInput(t *testing.T) {
	tests := []struct {
		input         string
		wantCandidate string
		wantRest      string
	}{
		{"auth-api/my-task", "auth-api", "my-task"},
		{"auth-api/my-task/part-1", "auth-api", "my-task/part-1"},
		{"feature/my-task", "feature", "my-task"},
		{"my-task", "", "my-task"},
		{"", "", ""},
	}
	for _, tt := range tests {
		candidate, rest := resolver.SplitServiceInput(tt.input)
		if candidate != tt.wantCandidate || rest != tt.wantRest {
			t.Errorf("SplitServiceInput(%q) = (%q, %q), want (%q, %q)", tt.input, candidate, rest, tt.wantCandidate, tt.wantRest)
		}
	}
}

func TestSanitizeTask(t *testing.T) {
	tests := []struct {
		input, want string
	}{
		{"my-task", "my-task"},
		{"auth-redirect/part-1", "auth-redirect-part-1"},
		{"a/b/c", "a-b-c"},
		{"no-slash", "no-slash"},
	}
	for _, tt := range tests {
		got := resolver.SanitizeTask(tt.input)
		if got != tt.want {
			t.Errorf("SanitizeTask(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestServiceFromBranch(t *testing.T) {
	prefixes := resolver.AllPrefixes()

	tests := []struct {
		branch     string
		wantSvc    string
		wantTask   string
		wantPrefix string
	}{
		{"feature/my-task", "", "my-task", featurePrefix},
		{"bugfix/login-fix", "", "login-fix", bugfixPrefix},
		{"auth-api/feature/my-task", "auth-api", "my-task", featurePrefix},
		{"auth-api/bugfix/crash", "auth-api", "crash", bugfixPrefix},
		{"bare-branch", "", "bare-branch", ""},
		{"unknown/prefix", "", "unknown/prefix", ""},
	}
	for _, tt := range tests {
		svc, task, prefix := resolver.ServiceFromBranch(tt.branch, prefixes)
		if svc != tt.wantSvc || task != tt.wantTask || prefix != tt.wantPrefix {
			t.Errorf("ServiceFromBranch(%q) = (%q, %q, %q), want (%q, %q, %q)",
				tt.branch, svc, task, prefix, tt.wantSvc, tt.wantTask, tt.wantPrefix)
		}
	}
}

func TestFindBranchForTaskWithService(t *testing.T) {
	worktrees := []resolver.WorktreeInfo{
		{Path: "/wt/feature-login", Branch: featurePrefix + taskLogin},
		{Path: "/wt/auth-api-feature-login", Branch: "auth-api/" + featurePrefix + taskLogin},
		{Path: "/wt/web-app-feature-login", Branch: "web-app/" + featurePrefix + taskLogin},
		{Path: "/wt/bugfix-crash", Branch: bugfixPrefix + taskCrash},
	}

	prefixes := resolver.AllPrefixes()

	t.Run("monorepo match with service", func(t *testing.T) {
		wt, ok := resolver.FindBranchForTask("auth-api", taskLogin, worktrees, prefixes)
		if !ok {
			t.Fatal(msgExpectedMatch)
		}
		if wt.Branch != "auth-api/"+featurePrefix+taskLogin {
			t.Errorf(fmtGotBranch, wt.Branch, "auth-api/"+featurePrefix+taskLogin)
		}
	})

	t.Run("standard match without service", func(t *testing.T) {
		wt, ok := resolver.FindBranchForTask("", taskCrash, worktrees, prefixes)
		if !ok {
			t.Fatal(msgExpectedMatch)
		}
		if wt.Branch != bugfixPrefix+taskCrash {
			t.Errorf(fmtGotBranch, wt.Branch, bugfixPrefix+taskCrash)
		}
	})

	t.Run("bare task finds standard match first", func(t *testing.T) {
		wt, ok := resolver.FindBranchForTask("", taskLogin, worktrees, prefixes)
		if !ok {
			t.Fatal(msgExpectedMatch)
		}
		// Should find the standard (non-service) branch first
		if wt.Branch != featurePrefix+taskLogin {
			t.Errorf(fmtGotBranch, wt.Branch, featurePrefix+taskLogin)
		}
	})

	t.Run("bare task with only monorepo matches and ambiguity", func(t *testing.T) {
		// Remove the standard worktree, keep only service-scoped ones
		monoOnly := []resolver.WorktreeInfo{
			{Path: "/wt/auth-api-feature-login", Branch: "auth-api/" + featurePrefix + taskLogin},
			{Path: "/wt/web-app-feature-login", Branch: "web-app/" + featurePrefix + taskLogin},
		}
		_, ok := resolver.FindBranchForTask("", taskLogin, monoOnly, prefixes)
		if ok {
			t.Fatal("expected no match due to ambiguity (2 services)")
		}
	})

	t.Run("bare task with single monorepo match", func(t *testing.T) {
		singleMono := []resolver.WorktreeInfo{
			{Path: "/wt/auth-api-feature-login", Branch: "auth-api/" + featurePrefix + taskLogin},
		}
		wt, ok := resolver.FindBranchForTask("", taskLogin, singleMono, prefixes)
		if !ok {
			t.Fatal(msgExpectedMatch)
		}
		if wt.Branch != "auth-api/"+featurePrefix+taskLogin {
			t.Errorf(fmtGotBranch, wt.Branch, "auth-api/"+featurePrefix+taskLogin)
		}
	})
}

func TestFindAllBranchesForTask(t *testing.T) {
	worktrees := []resolver.WorktreeInfo{
		{Path: "/wt/feature-login", Branch: featurePrefix + taskLogin},
		{Path: "/wt/auth-api-feature-login", Branch: "auth-api/" + featurePrefix + taskLogin},
		{Path: "/wt/web-app-feature-login", Branch: "web-app/" + featurePrefix + taskLogin},
	}
	prefixes := resolver.AllPrefixes()

	matches := resolver.FindAllBranchesForTask(taskLogin, worktrees, prefixes)
	if len(matches) != 3 {
		t.Fatalf("expected 3 matches, got %d", len(matches))
	}

	noMatches := resolver.FindAllBranchesForTask("nonexistent", worktrees, prefixes)
	if len(noMatches) != 0 {
		t.Fatalf("expected 0 matches, got %d", len(noMatches))
	}
}

func TestTaskAndType(t *testing.T) {
	prefixes := resolver.AllPrefixes()

	const svcAuthAPI = "auth-api"

	tests := []struct {
		branch       string
		wantTask     string
		wantTypeName string
	}{
		{"foo", "foo", ""},
		{featurePrefix + "foo", "foo", "feature"},
		{bugfixPrefix + "login-fix", "login-fix", "bugfix"},
		{svcAuthAPI + "/" + featurePrefix + taskLogin, taskLogin, "feature"},
		{"", "", ""},
	}
	for _, tt := range tests {
		task, typeName := resolver.TaskAndType(tt.branch, prefixes)
		if task != tt.wantTask {
			t.Errorf("TaskAndType(%q) task = %q, want %q", tt.branch, task, tt.wantTask)
		}
		if typeName != tt.wantTypeName {
			t.Errorf("TaskAndType(%q) typeName = %q, want %q", tt.branch, typeName, tt.wantTypeName)
		}
	}
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

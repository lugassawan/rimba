package operations

import (
	"context"
	"strings"
	"testing"

	"github.com/lugassawan/rimba/internal/config"
	"github.com/lugassawan/rimba/internal/resolver"
)

// testCustomPrefix and branchProj123 are shared fixtures for tests proving
// operations recognize a custom-prefixed branch from a context-carried
// resolver.PrefixSet (see #269).
const (
	testCustomPrefix = "PROJ-"
	branchProj123    = "PROJ-123"
)

// customPrefixContext returns a context carrying a config with a single
// custom [[resolver.prefix]] entry (no built-in aliasing), for exercising the
// config.PrefixSetFromContext funnel in operations tests.
func customPrefixContext() context.Context {
	cfg := &config.Config{Resolver: &config.ResolverConfig{Prefix: []config.PrefixEntry{{Prefix: testCustomPrefix}}}}
	return config.WithConfig(context.Background(), cfg)
}

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

	infos, err := ListWorktreeInfos(context.Background(), r)
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
	if _, err := ListWorktreeInfos(context.Background(), r); err == nil {
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
		wt, err := FindWorktree(context.Background(), r, "", "login")
		if err != nil {
			t.Fatalf("FindWorktree: %v", err)
		}
		if wt.Branch != branchFeature {
			t.Errorf("Branch = %q, want %q", wt.Branch, branchFeature)
		}
	})

	t.Run("not found", func(t *testing.T) {
		_, err := FindWorktree(context.Background(), r, "", "nonexistent")
		if err == nil {
			t.Fatal("expected error for missing worktree")
		}
		if !strings.Contains(err.Error(), "To fix:") {
			t.Errorf("worktree-not-found error missing hint, got: %v", err)
		}
		if !strings.Contains(err.Error(), "rimba list") {
			t.Errorf("worktree-not-found error missing 'rimba list' hint, got: %v", err)
		}
	})
}

func TestListWorktreeInfosCustomPrefix(t *testing.T) {
	porcelain := strings.Join([]string{
		"worktree /repo",
		"HEAD abc123",
		"branch refs/heads/main",
		"",
		"worktree /repo-worktrees/PROJ-123",
		"HEAD def456",
		"branch refs/heads/PROJ-123",
		"",
	}, "\n")

	r := &mockRunner{
		run:      func(_ ...string) (string, error) { return porcelain, nil },
		runInDir: noopRunInDir,
	}

	infos, err := ListWorktreeInfos(customPrefixContext(), r)
	if err != nil {
		t.Fatalf("ListWorktreeInfos: %v", err)
	}
	if len(infos) != 2 {
		t.Fatalf("got %d worktrees, want 2", len(infos))
	}
	if infos[1].Branch != branchProj123 {
		t.Errorf("infos[1].Branch = %q, want %q", infos[1].Branch, branchProj123)
	}
}

func TestFindWorktreeCustomPrefix(t *testing.T) {
	porcelain := strings.Join([]string{
		"worktree /repo",
		"HEAD abc123",
		"branch refs/heads/main",
		"",
		"worktree /repo-worktrees/PROJ-123",
		"HEAD def456",
		"branch refs/heads/PROJ-123",
		"",
	}, "\n")

	r := &mockRunner{
		run:      func(_ ...string) (string, error) { return porcelain, nil },
		runInDir: noopRunInDir,
	}

	wt, err := FindWorktree(customPrefixContext(), r, "", "123")
	if err != nil {
		t.Fatalf("FindWorktree with custom prefix: %v", err)
	}
	if wt.Branch != branchProj123 {
		t.Errorf("Branch = %q, want %q", wt.Branch, branchProj123)
	}

	// Parity: with no config in context, PrefixSetFromContext degrades to
	// built-ins only, so the custom-prefixed branch's stripped task ("123")
	// is not resolvable — behavior is byte-identical to pre-migration code.
	if _, err := FindWorktree(context.Background(), r, "", "123"); err == nil {
		t.Fatal("expected FindWorktree to fail without custom prefix config (built-ins-only parity)")
	}
}

func TestFindWorktreeAmbiguity(t *testing.T) {
	// Two service-scoped worktrees with the same task name
	porcelain := strings.Join([]string{
		"worktree /repo",
		"HEAD abc123",
		"branch refs/heads/main",
		"",
		"worktree /repo-worktrees/auth-api-feature-login",
		"HEAD def456",
		"branch refs/heads/auth-api/feature/login",
		"",
		"worktree /repo-worktrees/web-app-feature-login",
		"HEAD ghi789",
		"branch refs/heads/web-app/feature/login",
		"",
	}, "\n")

	r := &mockRunner{
		run:      func(_ ...string) (string, error) { return porcelain, nil },
		runInDir: noopRunInDir,
	}

	_, err := FindWorktree(context.Background(), r, "", "login")
	if err == nil {
		t.Fatal("expected ambiguity error")
	}
	if !strings.Contains(err.Error(), "multiple worktrees match") {
		t.Errorf("expected ambiguity error, got: %v", err)
	}
	if !strings.Contains(err.Error(), "auth-api/feature/login") {
		t.Errorf("expected error to list auth-api branch, got: %v", err)
	}
	if !strings.Contains(err.Error(), "web-app/feature/login") {
		t.Errorf("expected error to list web-app branch, got: %v", err)
	}
}

func TestFindWorktreeError(t *testing.T) {
	r := &mockRunner{
		run:      func(_ ...string) (string, error) { return "", errGitFailed },
		runInDir: noopRunInDir,
	}
	if _, err := FindWorktree(context.Background(), r, "", "login"); err == nil {
		t.Fatal("expected error")
	}
}

func TestFilterByType(t *testing.T) {
	ps := resolver.DefaultPrefixSet()
	worktrees := []resolver.WorktreeInfo{
		{Branch: branchFeature},
		{Branch: branchBugfixTypo},
		{Branch: branchMain},
		{Branch: "feature/signup"},
	}

	features := FilterByType(worktrees, ps, "feature")
	if len(features) != 2 {
		t.Fatalf("expected 2 feature worktrees, got %d", len(features))
	}
	if features[0].Branch != branchFeature {
		t.Errorf("features[0] = %q, want %q", features[0].Branch, branchFeature)
	}
	if features[1].Branch != "feature/signup" {
		t.Errorf("features[1] = %q, want %q", features[1].Branch, "feature/signup")
	}

	bugfixes := FilterByType(worktrees, ps, "bugfix")
	if len(bugfixes) != 1 {
		t.Fatalf("expected 1 bugfix worktree, got %d", len(bugfixes))
	}

	hotfixes := FilterByType(worktrees, ps, "hotfix")
	if len(hotfixes) != 0 {
		t.Errorf("expected 0 hotfix worktrees, got %d", len(hotfixes))
	}
}

func TestFilterByTypeCustomPrefixWithoutSlash(t *testing.T) {
	ps := resolver.NewPrefixSet([]resolver.PrefixSpec{{Prefix: testCustomPrefix}})
	worktrees := []resolver.WorktreeInfo{
		{Branch: branchProj123},
		{Branch: branchFeature},
	}

	matches := FilterByType(worktrees, ps, testCustomPrefix)
	if len(matches) != 1 {
		t.Fatalf("expected 1 match, got %d", len(matches))
	}
	if matches[0].Branch != branchProj123 {
		t.Errorf("matches[0] = %q, want %q", matches[0].Branch, branchProj123)
	}
}

func TestFilterByTypeUnrecognizedTypeReturnsEmpty(t *testing.T) {
	ps := resolver.DefaultPrefixSet()
	worktrees := []resolver.WorktreeInfo{{Branch: branchFeature}}

	if got := FilterByType(worktrees, ps, "nonexistent"); got != nil {
		t.Errorf("expected nil for unrecognized type, got %v", got)
	}
}

package cmd

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lugassawan/rimba/internal/config"
	"github.com/spf13/cobra"
)

// notGitRepoRunner returns a mockRunner that simulates running outside a git repository.
func notGitRepoRunner() *mockRunner {
	return &mockRunner{
		run:      func(_ ...string) (string, error) { return "", errors.New("not a git repository") },
		runInDir: noopRunInDir,
	}
}

// setupGlobalInit wires the HOME env to a temp dir and installs a notGitRepoRunner.
// Returns the home path and the restore func from overrideNewRunner.
func setupGlobalInit(t *testing.T) (home string, restore func()) {
	t.Helper()
	home = t.TempDir()
	t.Setenv("HOME", home)
	restore = overrideNewRunner(notGitRepoRunner())
	return
}

// repoRootRunner returns a mockRunner whose RepoRoot/MainRepoRoot resolves to dir.
func repoRootRunner(dir string, extra func(args ...string) (string, error)) *mockRunner {
	return &mockRunner{
		run: func(args ...string) (string, error) {
			if len(args) >= 2 && args[1] == cmdGitCommonDir {
				return filepath.Join(dir, ".git"), nil
			}
			if len(args) >= 2 && args[1] == "--show-toplevel" {
				return dir, nil
			}
			if extra != nil {
				return extra(args...)
			}
			return "", errors.New("unexpected")
		},
		runInDir: noopRunInDir,
	}
}

// TestResolveMainBranchIgnoresConfigDefaultSource is a regression test for
// issue #389: default_source is internal-only (toml:"-") and never round-trips
// through Save/Resolve, so a saved config can no longer short-circuit git-based
// branch detection.
func TestResolveMainBranchIgnoresConfigDefaultSource(t *testing.T) {
	dir := t.TempDir()
	cfg := &config.Config{WorktreeDir: "../worktrees", DefaultSource: "develop"}
	if err := config.Save(filepath.Join(dir, config.FileName), cfg); err != nil {
		t.Fatalf("Save config: %v", err)
	}

	r := repoRootRunner(dir, func(args ...string) (string, error) {
		if args[0] == cmdSymbolicRef {
			return refsRemotesOriginMain, nil
		}
		return "", errors.New("unexpected")
	})
	branch, err := resolveMainBranch(context.Background(), r)
	if err != nil {
		t.Fatalf("resolveMainBranch: %v", err)
	}
	if branch != branchMain {
		t.Errorf("branch = %q, want %q (config default_source must be ignored)", branch, branchMain)
	}
}

// TestResolveMainBranchIgnoresDirConfigDefaultSource is the .rimba/settings.toml
// variant of TestResolveMainBranchIgnoresConfigDefaultSource.
func TestResolveMainBranchIgnoresDirConfigDefaultSource(t *testing.T) {
	dir := t.TempDir()
	rimbaDir := filepath.Join(dir, config.DirName)
	if err := os.MkdirAll(rimbaDir, 0755); err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{WorktreeDir: "../worktrees", DefaultSource: "develop"}
	if err := config.Save(filepath.Join(rimbaDir, config.TeamFile), cfg); err != nil {
		t.Fatalf("Save config: %v", err)
	}

	r := repoRootRunner(dir, func(args ...string) (string, error) {
		if args[0] == cmdSymbolicRef {
			return refsRemotesOriginMain, nil
		}
		return "", errors.New("unexpected")
	})
	branch, err := resolveMainBranch(context.Background(), r)
	if err != nil {
		t.Fatalf("resolveMainBranch: %v", err)
	}
	if branch != branchMain {
		t.Errorf("branch = %q, want %q (config default_source must be ignored)", branch, branchMain)
	}
}

func TestResolveMainBranchFallback(t *testing.T) {
	dir := t.TempDir()

	r := repoRootRunner(dir, func(args ...string) (string, error) {
		if args[0] == cmdSymbolicRef {
			return refsRemotesOriginMain, nil
		}
		return "", errors.New("unexpected")
	})

	branch, err := resolveMainBranch(context.Background(), r)
	if err != nil {
		t.Fatalf("resolveMainBranch: %v", err)
	}
	if branch != branchMain {
		t.Errorf("branch = %q, want %q", branch, branchMain)
	}
}

func TestResolveMainBranchError(t *testing.T) {
	r := &mockRunner{
		run:      func(_ ...string) (string, error) { return "", errors.New("not a git repo") },
		runInDir: noopRunInDir,
	}
	if _, err := resolveMainBranch(context.Background(), r); err == nil {
		t.Fatal(errExpected)
	}
}

func TestListWorktreeInfos(t *testing.T) {
	porcelain := strings.Join([]string{
		"worktree /repo",
		"HEAD abc123",
		"branch refs/heads/main",
		"",
		"worktree /repo-worktrees/feature-login",
		"HEAD def456",
		branchRefPrefix + branchFeature,
		"",
	}, "\n")

	r := &mockRunner{
		run:      func(_ ...string) (string, error) { return porcelain, nil },
		runInDir: noopRunInDir,
	}

	infos, err := listWorktreeInfos(context.Background(), r)
	if err != nil {
		t.Fatalf("listWorktreeInfos: %v", err)
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
	if _, err := listWorktreeInfos(context.Background(), r); err == nil {
		t.Fatal(errExpected)
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
		branchRefPrefix + branchFeature,
		"",
	}, "\n")

	r := &mockRunner{
		run:      func(_ ...string) (string, error) { return porcelain, nil },
		runInDir: noopRunInDir,
	}

	t.Run("found", func(t *testing.T) {
		wt, err := findWorktree(context.Background(), r, "login")
		if err != nil {
			t.Fatalf("findWorktree: %v", err)
		}
		if wt.Branch != branchFeature {
			t.Errorf("Branch = %q, want %q", wt.Branch, branchFeature)
		}
	})

	t.Run("not found", func(t *testing.T) {
		if _, err := findWorktree(context.Background(), r, "nonexistent"); err == nil {
			t.Fatal("expected error for missing worktree")
		}
	})
}

func TestFindWorktreeError(t *testing.T) {
	r := &mockRunner{
		run:      func(_ ...string) (string, error) { return "", errGitFailed },
		runInDir: noopRunInDir,
	}
	if _, err := findWorktree(context.Background(), r, "login"); err == nil {
		t.Fatal(errExpected)
	}
}

func TestWithBestEffortConfigNoRepo(t *testing.T) {
	restore := overrideNewRunner(notGitRepoRunner())
	defer restore()

	cmd := &cobra.Command{Use: "status", Annotations: map[string]string{"skipConfig": "true"}}
	ctx := context.Background()
	cmd.SetContext(ctx)

	got := withBestEffortConfig(cmd)
	if got != ctx {
		t.Fatal("expected withBestEffortConfig to return the original context unchanged when no repo is present")
	}
	if config.FromContext(got) != nil {
		t.Error("expected no config in context when no repo is present")
	}
}

func TestWithBestEffortConfigNilContext(t *testing.T) {
	restore := overrideNewRunner(notGitRepoRunner())
	defer restore()

	cmd := &cobra.Command{Use: "status", Annotations: map[string]string{"skipConfig": "true"}}

	got := withBestEffortConfig(cmd)
	if got == nil {
		t.Fatal("expected non-nil context")
	}
}

func TestWithBestEffortConfigValidCustomPrefix(t *testing.T) {
	dir := t.TempDir()
	cfg := &config.Config{
		WorktreeDir: "../worktrees",
		Resolver: &config.ResolverConfig{
			Prefix: []config.PrefixEntry{{Prefix: "PROJ-"}},
		},
	}
	if err := config.Save(filepath.Join(dir, config.FileName), cfg); err != nil {
		t.Fatalf("Save config: %v", err)
	}

	// default_source is internal-only (toml:"-") and is always re-derived from git.
	r := repoRootRunner(dir, func(args ...string) (string, error) {
		if len(args) >= 1 && args[0] == cmdSymbolicRef {
			return refsRemotesOriginMain, nil
		}
		return "", errors.New("unexpected")
	})
	restore := overrideNewRunner(r)
	defer restore()

	cmd := &cobra.Command{Use: "status", Annotations: map[string]string{"skipConfig": "true"}}
	cmd.SetContext(context.Background())

	got := withBestEffortConfig(cmd)
	loaded := config.FromContext(got)
	if loaded == nil {
		t.Fatal("expected config to be loaded from a valid repo")
	}
	if !config.PrefixSetFromContext(got).HasCustom() {
		t.Error("expected PrefixSetFromContext to report HasCustom() true")
	}
}

func TestWithBestEffortConfigEmptyDefaultSourceDetectsBranch(t *testing.T) {
	dir := t.TempDir()
	cfg := &config.Config{
		WorktreeDir: "../worktrees",
		Resolver: &config.ResolverConfig{
			Prefix: []config.PrefixEntry{{Prefix: "PROJ-"}},
		},
	}
	if err := config.Save(filepath.Join(dir, config.FileName), cfg); err != nil {
		t.Fatalf("Save config: %v", err)
	}

	r := repoRootRunner(dir, func(args ...string) (string, error) {
		if len(args) >= 1 && args[0] == "symbolic-ref" {
			return "refs/remotes/origin/main", nil
		}
		return "", errors.New("unexpected")
	})
	restore := overrideNewRunner(r)
	defer restore()

	cmd := &cobra.Command{Use: "status", Annotations: map[string]string{"skipConfig": "true"}}
	cmd.SetContext(context.Background())

	got := withBestEffortConfig(cmd)
	loaded := config.FromContext(got)
	if loaded == nil {
		t.Fatal("expected config to be loaded when DefaultSource is empty and default branch detection succeeds")
	}
	if loaded.DefaultSource != branchMain {
		t.Errorf("DefaultSource = %q, want %q (auto-detected)", loaded.DefaultSource, branchMain)
	}
}

func TestWithBestEffortConfigAlreadyLoaded(t *testing.T) {
	existing := &config.Config{WorktreeDir: "../worktrees", DefaultSource: "main"}
	ctx := config.WithConfig(context.Background(), existing)

	cmd := &cobra.Command{Use: "status", Annotations: map[string]string{"skipConfig": "true"}}
	cmd.SetContext(ctx)

	got := withBestEffortConfig(cmd)
	if got != ctx {
		t.Error("expected withBestEffortConfig to return the original context unchanged when config is already loaded")
	}
}

func TestWithBestEffortConfigInvalidConfigSwallowed(t *testing.T) {
	dir := t.TempDir()
	// An invalid config: a resolver entry with an empty prefix fails Validate().
	cfg := &config.Config{
		WorktreeDir: "../worktrees",
		Resolver: &config.ResolverConfig{
			Prefix: []config.PrefixEntry{{Prefix: ""}},
		},
	}
	if err := config.Save(filepath.Join(dir, config.FileName), cfg); err != nil {
		t.Fatalf("Save config: %v", err)
	}

	// default_source is internal-only (toml:"-") and is always re-derived from
	// git, so branch detection must succeed here to exercise the Validate()
	// failure path (rather than short-circuiting earlier on git detection).
	r := repoRootRunner(dir, func(args ...string) (string, error) {
		if len(args) >= 1 && args[0] == cmdSymbolicRef {
			return refsRemotesOriginMain, nil
		}
		return "", errors.New("unexpected")
	})
	restore := overrideNewRunner(r)
	defer restore()

	cmd := &cobra.Command{Use: "status", Annotations: map[string]string{"skipConfig": "true"}}
	ctx := context.Background()
	cmd.SetContext(ctx)

	got := withBestEffortConfig(cmd)
	if got != ctx {
		t.Error("expected withBestEffortConfig to return the original context unchanged when config is invalid")
	}
}

func TestHintPainter(t *testing.T) {
	prev := os.Getenv("NO_COLOR")
	os.Unsetenv("NO_COLOR")
	defer func() {
		if prev != "" {
			os.Setenv("NO_COLOR", prev)
		}
	}()

	cmd, _ := newTestCmd()
	p := hintPainter(cmd)
	got := p.Paint("hello", "\033[31m")
	if got != "hello" {
		t.Errorf("expected uncolored output, got %q", got)
	}
}

func TestSpinnerOpts(t *testing.T) {
	cmd, buf := newTestCmd()
	opts := spinnerOpts(cmd)

	if !opts.NoColor {
		t.Error("expected NoColor=true from --no-color flag")
	}
	if opts.Writer != buf {
		t.Error("expected Writer to be the command's stderr buffer")
	}
}

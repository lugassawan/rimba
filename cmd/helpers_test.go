package cmd

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lugassawan/rimba/internal/config"
)

// repoRootRunner returns a mockRunner whose RepoRoot resolves to dir.
func repoRootRunner(dir string, extra func(args ...string) (string, error)) *mockRunner {
	return &mockRunner{
		run: func(args ...string) (string, error) {
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

func TestResolveMainBranchFromConfig(t *testing.T) {
	dir := t.TempDir()
	cfg := &config.Config{WorktreeDir: "../worktrees", DefaultSource: "develop"}
	if err := config.Save(filepath.Join(dir, config.FileName), cfg); err != nil {
		t.Fatalf("Save config: %v", err)
	}

	r := repoRootRunner(dir, nil)
	branch, err := resolveMainBranch(r)
	if err != nil {
		t.Fatalf("resolveMainBranch: %v", err)
	}
	if branch != "develop" {
		t.Errorf("branch = %q, want %q", branch, "develop")
	}
}

func TestResolveMainBranchFallback(t *testing.T) {
	dir := t.TempDir()

	r := repoRootRunner(dir, func(args ...string) (string, error) {
		if args[0] == "symbolic-ref" {
			return "refs/remotes/origin/main", nil
		}
		return "", errors.New("unexpected")
	})

	branch, err := resolveMainBranch(r)
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
	if _, err := resolveMainBranch(r); err == nil {
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
		"branch refs/heads/" + branchFeature,
		"",
	}, "\n")

	r := &mockRunner{
		run:      func(_ ...string) (string, error) { return porcelain, nil },
		runInDir: noopRunInDir,
	}

	infos, err := listWorktreeInfos(r)
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
	if _, err := listWorktreeInfos(r); err == nil {
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
		"branch refs/heads/" + branchFeature,
		"",
	}, "\n")

	r := &mockRunner{
		run:      func(_ ...string) (string, error) { return porcelain, nil },
		runInDir: noopRunInDir,
	}

	t.Run("found", func(t *testing.T) {
		wt, err := findWorktree(r, "login")
		if err != nil {
			t.Fatalf("findWorktree: %v", err)
		}
		if wt.Branch != branchFeature {
			t.Errorf("Branch = %q, want %q", wt.Branch, branchFeature)
		}
	})

	t.Run("not found", func(t *testing.T) {
		if _, err := findWorktree(r, "nonexistent"); err == nil {
			t.Fatal("expected error for missing worktree")
		}
	})
}

func TestFindWorktreeError(t *testing.T) {
	r := &mockRunner{
		run:      func(_ ...string) (string, error) { return "", errGitFailed },
		runInDir: noopRunInDir,
	}
	if _, err := findWorktree(r, "login"); err == nil {
		t.Fatal(errExpected)
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

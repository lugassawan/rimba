package cmd

import (
	"context"
	"strings"
	"testing"

	"github.com/lugassawan/rimba/internal/config"
)

func TestFindArchivedBranch(t *testing.T) {
	mr := &mockRunner{
		run: func(args ...string) (string, error) {
			switch {
			case args[0] == cmdBranch:
				return branchListArchived, nil
			case args[0] == cmdWorktreeTest && args[1] == cmdList:
				return wtRepo + headMainBlock + "\n" +
					"worktree /wt/feature-active-task\nHEAD abc\nbranch refs/heads/feature/active-task\n", nil
			}
			return "", nil
		},
		runInDir: noopRunInDir,
	}

	branch, err := findArchivedBranch(mr, "archived-task")
	if err != nil {
		t.Fatalf("findArchivedBranch: %v", err)
	}
	if branch != "feature/archived-task" {
		t.Errorf("branch = %q, want %q", branch, "feature/archived-task")
	}
}

func TestFindArchivedBranchNotFound(t *testing.T) {
	mr := &mockRunner{
		run: func(args ...string) (string, error) {
			switch {
			case args[0] == cmdBranch:
				return "main\nfeature/active-task", nil
			case args[0] == cmdWorktreeTest && args[1] == cmdList:
				return wtRepo + headMainBlock + "\n" +
					"worktree /wt/feature-active-task\nHEAD abc\nbranch refs/heads/feature/active-task\n", nil
			}
			return "", nil
		},
		runInDir: noopRunInDir,
	}

	_, err := findArchivedBranch(mr, "nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent archived branch")
	}
}

func TestFindArchivedBranchExactMatch(t *testing.T) {
	mr := &mockRunner{
		run: func(args ...string) (string, error) {
			switch {
			case args[0] == cmdBranch:
				return "main\nmy-custom-branch", nil
			case args[0] == cmdWorktreeTest && args[1] == cmdList:
				return wtRepo + headMainBlock, nil
			}
			return "", nil
		},
		runInDir: noopRunInDir,
	}

	branch, err := findArchivedBranch(mr, "my-custom-branch")
	if err != nil {
		t.Fatalf("findArchivedBranch: %v", err)
	}
	if branch != "my-custom-branch" {
		t.Errorf("branch = %q, want %q", branch, "my-custom-branch")
	}
}

func TestFindArchivedBranchByTaskExtraction(t *testing.T) {
	mr := &mockRunner{
		run: func(args ...string) (string, error) {
			switch {
			case args[0] == cmdBranch:
				return "main\nbugfix/some-task", nil
			case args[0] == cmdWorktreeTest && args[1] == cmdList:
				return wtRepo + headMainBlock, nil
			}
			return "", nil
		},
		runInDir: noopRunInDir,
	}

	// "some-task" won't match prefix+task for "feature/some-task" or exact "some-task",
	// but will match via task extraction from "bugfix/some-task"
	branch, err := findArchivedBranch(mr, "some-task")
	if err != nil {
		t.Fatalf("findArchivedBranch: %v", err)
	}
	if branch != "bugfix/some-task" {
		t.Errorf("branch = %q, want %q", branch, "bugfix/some-task")
	}
}

func TestFindArchivedBranchError(t *testing.T) {
	mr := &mockRunner{
		run: func(args ...string) (string, error) {
			if args[0] == cmdBranch {
				return "", errGitFailed
			}
			return "", nil
		},
		runInDir: noopRunInDir,
	}

	_, err := findArchivedBranch(mr, "any")
	if err == nil {
		t.Fatal("expected error from LocalBranches failure")
	}
}

func TestFindArchivedBranchWorktreeError(t *testing.T) {
	mr := &mockRunner{
		run: func(args ...string) (string, error) {
			if args[0] == cmdBranch {
				return "main\nfeature/task", nil
			}
			if args[0] == cmdWorktreeTest {
				return "", errGitFailed
			}
			return "", nil
		},
		runInDir: noopRunInDir,
	}

	_, err := findArchivedBranch(mr, "task")
	if err == nil {
		t.Fatal("expected error from ListWorktrees failure")
	}
}

func TestFindArchivedBranches(t *testing.T) {
	mr := &mockRunner{
		run: func(args ...string) (string, error) {
			switch {
			case args[0] == cmdBranch:
				return "main\nfeature/archived\nfeature/active", nil
			case args[0] == cmdWorktreeTest && args[1] == cmdList:
				return strings.Join([]string{
					wtRepo, headABC123, branchRefMain, "",
					"worktree /wt/feature-active", "HEAD def456", "branch refs/heads/feature/active", "",
				}, "\n"), nil
			}
			return "", nil
		},
		runInDir: noopRunInDir,
	}

	archived, err := findArchivedBranches(mr, branchMain)
	if err != nil {
		t.Fatalf("findArchivedBranches: %v", err)
	}
	if len(archived) != 1 {
		t.Fatalf("got %d branches, want 1", len(archived))
	}
	if archived[0] != "feature/archived" {
		t.Errorf("branch = %q, want %q", archived[0], "feature/archived")
	}
}

func TestFindArchivedBranchesError(t *testing.T) {
	mr := &mockRunner{
		run: func(args ...string) (string, error) {
			if args[0] == cmdBranch {
				return "", errGitFailed
			}
			return "", nil
		},
		runInDir: noopRunInDir,
	}

	_, err := findArchivedBranches(mr, branchMain)
	if err == nil {
		t.Fatal("expected error from LocalBranches failure")
	}
}

func TestFindArchivedBranchesWorktreeError(t *testing.T) {
	mr := &mockRunner{
		run: func(args ...string) (string, error) {
			if args[0] == cmdBranch {
				return "main\nfeature/task", nil
			}
			if args[0] == cmdWorktreeTest {
				return "", errGitFailed
			}
			return "", nil
		},
		runInDir: noopRunInDir,
	}

	_, err := findArchivedBranches(mr, branchMain)
	if err == nil {
		t.Fatal("expected error from ListWorktrees failure")
	}
}

func TestRestoreSuccess(t *testing.T) {
	restore := overrideNewRunner(&mockRunner{
		run: func(args ...string) (string, error) {
			switch {
			case args[0] == cmdRevParse && args[1] == cmdShowToplevel:
				return repoPath, nil
			case args[0] == cmdBranch:
				return "main\nfeature/restored-task", nil
			case args[0] == cmdWorktreeTest && args[1] == cmdList:
				return wtRepo + headMainBlock, nil
			case args[0] == cmdWorktreeTest && args[1] == "add":
				return "", nil
			}
			return "", nil
		},
		runInDir: noopRunInDir,
	})
	defer restore()

	cmd, buf := newTestCmd()
	cmd.SetContext(config.WithConfig(context.Background(), &config.Config{
		WorktreeDir:   defaultRelativeWtDir,
		DefaultSource: branchMain,
	}))

	// Set skip flags
	cmd.Flags().Bool(flagSkipDeps, false, "")
	cmd.Flags().Bool(flagSkipHooks, false, "")
	_ = cmd.Flags().Set(flagSkipDeps, "true")
	_ = cmd.Flags().Set(flagSkipHooks, "true")

	if err := restoreCmd.RunE(cmd, []string{"restored-task"}); err != nil {
		t.Fatalf("restoreCmd.RunE: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "Restored worktree") {
		t.Errorf("expected 'Restored worktree', got: %q", output)
	}
}

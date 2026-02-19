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
				return "main\nfeature/archived-task\nfeature/active-task", nil
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

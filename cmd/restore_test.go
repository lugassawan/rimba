package cmd

import (
	"context"
	"strings"
	"testing"

	"github.com/lugassawan/rimba/internal/config"
)

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

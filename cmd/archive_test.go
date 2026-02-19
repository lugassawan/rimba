package cmd

import (
	"strings"
	"testing"
)

func TestArchiveSuccess(t *testing.T) {
	restore := overrideNewRunner(&mockRunner{
		run: func(args ...string) (string, error) {
			switch {
			case args[0] == cmdRevParse && args[1] == cmdShowToplevel:
				return repoPath, nil
			case args[0] == cmdWorktreeTest && args[1] == cmdList:
				return wtRepo + headMainBlock + "\n" +
					wtFeatureLogin + "\n" + headDEF456 + "\n" + branchRefFeatureLogin + "\n", nil
			case args[0] == cmdWorktreeTest && args[1] == cmdRemove:
				return "", nil
			}
			return "", nil
		},
		runInDir: noopRunInDir,
	})
	defer restore()

	cmd, buf := newTestCmd()
	if err := archiveCmd.RunE(cmd, []string{taskLogin}); err != nil {
		t.Fatalf("archiveCmd.RunE: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "Archived worktree") {
		t.Errorf("expected 'Archived worktree', got: %q", output)
	}
	if !strings.Contains(output, "Branch preserved") {
		t.Errorf("expected 'Branch preserved', got: %q", output)
	}
}

func TestArchiveNotFound(t *testing.T) {
	restore := overrideNewRunner(&mockRunner{
		run: func(args ...string) (string, error) {
			switch {
			case args[0] == cmdRevParse && args[1] == cmdShowToplevel:
				return repoPath, nil
			case args[0] == cmdWorktreeTest && args[1] == cmdList:
				return wtRepo + headMainBlock, nil
			}
			return "", nil
		},
		runInDir: noopRunInDir,
	})
	defer restore()

	cmd, _ := newTestCmd()
	err := archiveCmd.RunE(cmd, []string{"nonexistent"})
	if err == nil {
		t.Fatal("expected error for nonexistent task")
	}
}

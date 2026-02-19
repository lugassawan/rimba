package cmd

import (
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestStatusNoWorktrees(t *testing.T) {
	restore := overrideNewRunner(&mockRunner{
		run: func(args ...string) (string, error) {
			switch {
			case args[0] == cmdRevParse && args[1] == cmdShowToplevel:
				return repoPath, nil
			case args[0] == cmdSymbolicRef:
				return refsRemotesOriginMain, nil
			case args[0] == cmdWorktreeTest && args[1] == cmdList:
				return wtRepo + headMainBlock, nil
			}
			return "", nil
		},
		runInDir: noopRunInDir,
	})
	defer restore()

	cmd, buf := newTestCmd()
	if err := statusCmd.RunE(cmd, nil); err != nil {
		t.Fatalf("statusCmd.RunE: %v", err)
	}

	if !strings.Contains(buf.String(), "No worktrees found") {
		t.Errorf("expected 'No worktrees found', got: %q", buf.String())
	}
}

func TestStatusWithWorktrees(t *testing.T) {
	restore := overrideNewRunner(&mockRunner{
		run: func(args ...string) (string, error) {
			switch {
			case args[0] == cmdRevParse && args[1] == cmdShowToplevel:
				return repoPath, nil
			case args[0] == cmdSymbolicRef:
				return refsRemotesOriginMain, nil
			case args[0] == cmdWorktreeTest && args[1] == cmdList:
				return wtRepo + headMainBlock + "\n" +
					wtFeatureLogin + "\n" + headDEF456 + "\n" + branchRefFeatureLogin + "\n", nil
			case args[0] == cmdLog:
				return "1700000000", nil
			}
			return "", nil
		},
		runInDir: func(dir string, args ...string) (string, error) {
			if args[0] == cmdStatus {
				return "", nil
			}
			if args[0] == cmdRevList {
				return aheadBehindZero, nil
			}
			return "", nil
		},
	})
	defer restore()

	cmd, buf := newTestCmd()
	if err := statusCmd.RunE(cmd, nil); err != nil {
		t.Fatalf("statusCmd.RunE: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "Worktrees:") {
		t.Errorf("expected 'Worktrees:' header, got: %q", output)
	}
	if !strings.Contains(output, taskLogin) {
		t.Errorf("expected task 'login', got: %q", output)
	}
}

func TestColorCount(t *testing.T) {
	cmd := &cobra.Command{}
	cmd.Flags().Bool(flagNoColor, true, "")
	p := hintPainter(cmd)
	// Zero count should not be colored
	s := colorCount(p, 0, "")
	if s != "0" {
		t.Errorf("colorCount(0) = %q, want %q", s, "0")
	}
}

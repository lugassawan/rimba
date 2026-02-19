package cmd

import (
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestLogNoWorktrees(t *testing.T) {
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
	cmd.Flags().Int(flagLimit, 0, "")
	cmd.Flags().String(flagSince, "", "")

	if err := logCmd.RunE(cmd, nil); err != nil {
		t.Fatalf("logCmd.RunE: %v", err)
	}

	if !strings.Contains(buf.String(), "No worktrees found") {
		t.Errorf("expected 'No worktrees found', got: %q", buf.String())
	}
}

func TestLogWithWorktrees(t *testing.T) {
	ts := strconv.FormatInt(time.Now().Add(-2*time.Hour).Unix(), 10)

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
				return ts + "\tfix login bug", nil
			}
			return "", nil
		},
		runInDir: noopRunInDir,
	})
	defer restore()

	cmd, buf := newTestCmd()
	cmd.Flags().Int(flagLimit, 0, "")
	cmd.Flags().String(flagSince, "", "")

	if err := logCmd.RunE(cmd, nil); err != nil {
		t.Fatalf("logCmd.RunE: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "Recent commits") {
		t.Errorf("expected 'Recent commits' header, got: %q", output)
	}
	if !strings.Contains(output, taskLogin) {
		t.Errorf("expected task 'login', got: %q", output)
	}
	if !strings.Contains(output, "fix login bug") {
		t.Errorf("expected commit subject, got: %q", output)
	}
}

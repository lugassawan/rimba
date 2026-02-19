package cmd

import (
	"errors"
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

func logRunnerWithWorktree(logResp func(branch string) (string, error)) *mockRunner {
	return &mockRunner{
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
				branch := args[len(args)-1]
				return logResp(branch)
			}
			return "", nil
		},
		runInDir: noopRunInDir,
	}
}

func TestLogWithInvalidSince(t *testing.T) {
	ts := strconv.FormatInt(time.Now().Add(-2*time.Hour).Unix(), 10)
	restore := overrideNewRunner(logRunnerWithWorktree(func(_ string) (string, error) {
		return ts + "\tcommit msg", nil
	}))
	defer restore()

	cmd, _ := newTestCmd()
	cmd.Flags().Int(flagLimit, 0, "")
	cmd.Flags().String(flagSince, "", "")
	_ = cmd.Flags().Set(flagSince, "invalid")

	err := logCmd.RunE(cmd, nil)
	if err == nil {
		t.Fatal("expected error for invalid --since value")
	}
	if !strings.Contains(err.Error(), "invalid --since") {
		t.Errorf("error = %q, want 'invalid --since'", err.Error())
	}
}

func TestLogWithSinceFilter(t *testing.T) {
	// Commit from 2 hours ago — should be included with "7d" but excluded with "1m"
	ts := strconv.FormatInt(time.Now().Add(-2*time.Hour).Unix(), 10)
	restore := overrideNewRunner(logRunnerWithWorktree(func(_ string) (string, error) {
		return ts + "\trecent commit", nil
	}))
	defer restore()

	cmd, buf := newTestCmd()
	cmd.Flags().Int(flagLimit, 0, "")
	cmd.Flags().String(flagSince, "", "")
	_ = cmd.Flags().Set(flagSince, "7d")

	if err := logCmd.RunE(cmd, nil); err != nil {
		t.Fatalf("logCmd.RunE: %v", err)
	}
	if !strings.Contains(buf.String(), "recent commit") {
		t.Errorf("expected commit within 7d window, got: %q", buf.String())
	}
}

func TestLogWithSinceFilterExcludes(t *testing.T) {
	// Commit from 30 days ago — should be excluded with "1d"
	ts := strconv.FormatInt(time.Now().Add(-30*24*time.Hour).Unix(), 10)
	restore := overrideNewRunner(logRunnerWithWorktree(func(_ string) (string, error) {
		return ts + "\told commit", nil
	}))
	defer restore()

	cmd, buf := newTestCmd()
	cmd.Flags().Int(flagLimit, 0, "")
	cmd.Flags().String(flagSince, "", "")
	_ = cmd.Flags().Set(flagSince, "1d")

	if err := logCmd.RunE(cmd, nil); err != nil {
		t.Fatalf("logCmd.RunE: %v", err)
	}
	if !strings.Contains(buf.String(), "No recent commits found") {
		t.Errorf("expected 'No recent commits found', got: %q", buf.String())
	}
}

func TestLogWithLimit(t *testing.T) {
	ts := strconv.FormatInt(time.Now().Add(-1*time.Hour).Unix(), 10)

	// Two worktrees with commits
	restore := overrideNewRunner(&mockRunner{
		run: func(args ...string) (string, error) {
			switch {
			case args[0] == cmdRevParse && args[1] == cmdShowToplevel:
				return repoPath, nil
			case args[0] == cmdSymbolicRef:
				return refsRemotesOriginMain, nil
			case args[0] == cmdWorktreeTest && args[1] == cmdList:
				return strings.Join([]string{
					wtRepo + headMainBlock,
					wtFeatureLogin, headDEF456, branchRefFeatureLogin, "",
					"worktree /wt/bugfix-typo", "HEAD ghi789", "branch refs/heads/" + branchBugfixTypo, "",
				}, "\n"), nil
			case args[0] == cmdLog:
				return ts + "\tcommit msg", nil
			}
			return "", nil
		},
		runInDir: noopRunInDir,
	})
	defer restore()

	cmd, buf := newTestCmd()
	cmd.Flags().Int(flagLimit, 0, "")
	cmd.Flags().String(flagSince, "", "")
	_ = cmd.Flags().Set(flagLimit, "1")

	if err := logCmd.RunE(cmd, nil); err != nil {
		t.Fatalf("logCmd.RunE: %v", err)
	}
	output := buf.String()
	if !strings.Contains(output, "1 worktree(s)") {
		t.Errorf("expected '1 worktree(s)' with limit=1, got: %q", output)
	}
}

func TestLogCommitInfoError(t *testing.T) {
	restore := overrideNewRunner(logRunnerWithWorktree(func(_ string) (string, error) {
		return "", errors.New("no commits")
	}))
	defer restore()

	cmd, buf := newTestCmd()
	cmd.Flags().Int(flagLimit, 0, "")
	cmd.Flags().String(flagSince, "", "")

	if err := logCmd.RunE(cmd, nil); err != nil {
		t.Fatalf("logCmd.RunE: %v", err)
	}
	if !strings.Contains(buf.String(), "No recent commits found") {
		t.Errorf("expected 'No recent commits found' when all entries fail, got: %q", buf.String())
	}
}

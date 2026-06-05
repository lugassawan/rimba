package cmd

import (
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/lugassawan/rimba/internal/output"
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

// findLogItem searches a JSON data slice for the entry with the given task name.
func findLogItem(t *testing.T, data []any, task string) map[string]any {
	t.Helper()
	for _, d := range data {
		if m, ok := d.(map[string]any); ok && m["task"] == task {
			return m
		}
	}
	t.Fatalf("%q entry not found in JSON output", task)
	return nil
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

func TestLogWithServiceColumn(t *testing.T) {
	ts := strconv.FormatInt(time.Now().Add(-1*time.Hour).Unix(), 10)

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
					"worktree /wt/auth-api-feature-login", headDEF456, "branch refs/heads/auth-api/feature/login", "",
				}, "\n"), nil
			case args[0] == cmdLog:
				return ts + "\tadd login", nil
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
	if !strings.Contains(output, "SERVICE") {
		t.Errorf("expected SERVICE column, got: %q", output)
	}
	if !strings.Contains(output, "auth-api") {
		t.Errorf("expected auth-api service, got: %q", output)
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

func TestLogJSON(t *testing.T) {
	ts := strconv.FormatInt(time.Now().Add(-2*time.Hour).Unix(), 10)
	restore := overrideNewRunner(logRunnerWithWorktree(func(_ string) (string, error) {
		return ts + "\tadd login feature", nil
	}))
	defer restore()

	cmd, buf := newTestCmd()
	_ = cmd.Flags().Set(flagJSON, "true")
	cmd.Flags().Int(flagLimit, 0, "")
	cmd.Flags().String(flagSince, "", "")

	if err := logCmd.RunE(cmd, nil); err != nil {
		t.Fatalf("logCmd.RunE: %v", err)
	}

	var env output.Envelope
	if err := json.Unmarshal(buf.Bytes(), &env); err != nil {
		t.Fatalf("invalid JSON: %v\noutput: %s", err, buf.String())
	}
	if env.Command != cmdLog {
		t.Errorf("command = %q, want %q", env.Command, cmdLog)
	}

	data, ok := env.Data.([]any)
	if !ok {
		t.Fatalf("data type = %T, want []any", env.Data)
	}
	if len(data) == 0 {
		t.Fatal("expected at least one log entry")
	}
	item := findLogItem(t, data, taskLogin)
	for _, field := range []string{"task", "type", "branch", "path", "last_commit", "subject"} {
		if _, exists := item[field]; !exists {
			t.Errorf("item missing field %q", field)
		}
	}
	if item["task"] != taskLogin {
		t.Errorf("task = %v, want %q", item["task"], taskLogin)
	}
	if item["path"] != pathWtFeatureLogin {
		t.Errorf("path = %v, want %q", item["path"], pathWtFeatureLogin)
	}
	if item["subject"] != "add login feature" {
		t.Errorf("subject = %v, want %q", item["subject"], "add login feature")
	}
	lc, _ := item["last_commit"].(string)
	if _, err := time.Parse(time.RFC3339, lc); err != nil {
		t.Errorf("last_commit %q is not RFC3339: %v", lc, err)
	}
}

func TestLogJSONEmpty(t *testing.T) {
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
	_ = cmd.Flags().Set(flagJSON, "true")
	cmd.Flags().Int(flagLimit, 0, "")
	cmd.Flags().String(flagSince, "", "")

	if err := logCmd.RunE(cmd, nil); err != nil {
		t.Fatalf("logCmd.RunE: %v", err)
	}

	var env output.Envelope
	if err := json.Unmarshal(buf.Bytes(), &env); err != nil {
		t.Fatalf("invalid JSON: %v\noutput: %s", err, buf.String())
	}
	if env.Command != cmdLog {
		t.Errorf("command = %q, want %q", env.Command, cmdLog)
	}

	data, ok := env.Data.([]any)
	if !ok {
		t.Fatalf("data type = %T, want []any", env.Data)
	}
	if len(data) != 0 {
		t.Errorf("expected empty data array, got %d items", len(data))
	}
}

// TestLogJSONEmptyAfterFilter covers the post-filter JSON path (guard 2 in RunE):
// worktrees exist but all entries are filtered out by --since.
func TestLogJSONEmptyAfterFilter(t *testing.T) {
	ts := strconv.FormatInt(time.Now().Add(-30*24*time.Hour).Unix(), 10)
	restore := overrideNewRunner(logRunnerWithWorktree(func(_ string) (string, error) {
		return ts + "\told commit", nil
	}))
	defer restore()

	cmd, buf := newTestCmd()
	_ = cmd.Flags().Set(flagJSON, "true")
	cmd.Flags().Int(flagLimit, 0, "")
	cmd.Flags().String(flagSince, "", "")
	_ = cmd.Flags().Set(flagSince, "1h")

	if err := logCmd.RunE(cmd, nil); err != nil {
		t.Fatalf("logCmd.RunE: %v", err)
	}

	var env output.Envelope
	if err := json.Unmarshal(buf.Bytes(), &env); err != nil {
		t.Fatalf("invalid JSON: %v\noutput: %s", err, buf.String())
	}
	if env.Command != cmdLog {
		t.Errorf("command = %q, want %q", env.Command, cmdLog)
	}

	data, ok := env.Data.([]any)
	if !ok {
		t.Fatalf("data type = %T, want []any", env.Data)
	}
	if len(data) != 0 {
		t.Errorf("expected empty data array after filter, got %d items", len(data))
	}
}

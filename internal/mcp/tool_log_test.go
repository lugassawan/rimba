package mcp

import (
	"errors"
	"strconv"
	"strings"
	"testing"
	"time"
)

func logMockRunner(worktreeOut string, logResp func(branch string) (string, error)) *mockRunner {
	return &mockRunner{
		run: func(args ...string) (string, error) {
			switch {
			case len(args) > 0 && args[0] == gitSymbolicRef:
				return refsOriginMain, nil
			case len(args) >= 2 && args[0] == gitWorktree && args[1] == gitList:
				return worktreeOut, nil
			case len(args) > 0 && args[0] == "log":
				branch := args[len(args)-1]
				return logResp(branch)
			}
			return "", nil
		},
	}
}

func TestLogToolNoWorktrees(t *testing.T) {
	worktreeOut := worktreePorcelain(struct{ path, branch string }{"/repo", "main"})
	r := logMockRunner(worktreeOut, nil)
	hctx := testContext(r)
	handler := handleLog(hctx)

	result := callTool(t, handler, nil)
	data := unmarshalJSON[logResult](t, result)
	if len(data.Entries) != 0 {
		t.Errorf("expected 0 entries, got %d", len(data.Entries))
	}
}

func TestLogToolHappyPath(t *testing.T) {
	ts := strconv.FormatInt(time.Now().Add(-2*time.Hour).Unix(), 10)
	worktreeOut := worktreePorcelain(
		struct{ path, branch string }{"/repo", "main"},
		struct{ path, branch string }{pathRepoWorktree + "feature-my-task", branchFeatureMyTask},
	)
	r := logMockRunner(worktreeOut, func(_ string) (string, error) {
		return ts + "\tfix login bug", nil
	})
	hctx := testContext(r)
	handler := handleLog(hctx)

	result := callTool(t, handler, nil)
	data := unmarshalJSON[logResult](t, result)
	if len(data.Entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(data.Entries))
	}
	entry := data.Entries[0]
	if entry.Task != taskMyTask {
		t.Errorf("task = %q, want %q", entry.Task, taskMyTask)
	}
	if entry.Branch != branchFeatureMyTask {
		t.Errorf("branch = %q, want %q", entry.Branch, branchFeatureMyTask)
	}
	if entry.Type != "feature" {
		t.Errorf("type = %q, want %q", entry.Type, "feature")
	}
	if entry.Subject != "fix login bug" {
		t.Errorf("subject = %q, want %q", entry.Subject, "fix login bug")
	}
}

// TestLogToolWorksWithoutConfig locks in the CLI's skipConfig behavior: log
// must still return entries when Config == nil.
func TestLogToolWorksWithoutConfig(t *testing.T) {
	ts := strconv.FormatInt(time.Now().Add(-1*time.Hour).Unix(), 10)
	worktreeOut := worktreePorcelain(
		struct{ path, branch string }{"/repo", "main"},
		struct{ path, branch string }{pathRepoWorktree + "feature-my-task", branchFeatureMyTask},
	)
	r := logMockRunner(worktreeOut, func(_ string) (string, error) {
		return ts + "\tcommit msg", nil
	})
	hctx := &HandlerContext{
		Runner:   r,
		Config:   nil,
		RepoRoot: "/repo",
		Version:  "test",
	}
	handler := handleLog(hctx)

	result := callTool(t, handler, nil)
	data := unmarshalJSON[logResult](t, result)
	if len(data.Entries) != 1 {
		t.Fatalf("expected 1 entry with nil config, got %d", len(data.Entries))
	}
	if data.Entries[0].Task != taskMyTask {
		t.Errorf("task = %q, want %q", data.Entries[0].Task, taskMyTask)
	}
}

func TestLogToolInvalidSince(t *testing.T) {
	worktreeOut := worktreePorcelain(
		struct{ path, branch string }{"/repo", "main"},
		struct{ path, branch string }{pathRepoWorktree + "feature-my-task", branchFeatureMyTask},
	)
	ts := strconv.FormatInt(time.Now().Unix(), 10)
	r := logMockRunner(worktreeOut, func(_ string) (string, error) {
		return ts + "\tcommit msg", nil
	})
	hctx := testContext(r)
	handler := handleLog(hctx)

	result := callTool(t, handler, map[string]any{"since": "invalid"})
	errText := resultError(t, result)
	if !strings.Contains(errText, "invalid since value") {
		t.Errorf("expected 'invalid since value' error, got: %s", errText)
	}
}

func TestLogToolSinceFilterExcludes(t *testing.T) {
	ts := strconv.FormatInt(time.Now().Add(-30*24*time.Hour).Unix(), 10)
	worktreeOut := worktreePorcelain(
		struct{ path, branch string }{"/repo", "main"},
		struct{ path, branch string }{pathRepoWorktree + "feature-my-task", branchFeatureMyTask},
	)
	r := logMockRunner(worktreeOut, func(_ string) (string, error) {
		return ts + "\told commit", nil
	})
	hctx := testContext(r)
	handler := handleLog(hctx)

	result := callTool(t, handler, map[string]any{"since": "1d"})
	data := unmarshalJSON[logResult](t, result)
	if len(data.Entries) != 0 {
		t.Errorf("expected 0 entries outside the since window, got %d", len(data.Entries))
	}
}

func TestLogToolSinceFilterIncludes(t *testing.T) {
	ts := strconv.FormatInt(time.Now().Add(-2*time.Hour).Unix(), 10)
	worktreeOut := worktreePorcelain(
		struct{ path, branch string }{"/repo", "main"},
		struct{ path, branch string }{pathRepoWorktree + "feature-my-task", branchFeatureMyTask},
	)
	r := logMockRunner(worktreeOut, func(_ string) (string, error) {
		return ts + "\trecent commit", nil
	})
	hctx := testContext(r)
	handler := handleLog(hctx)

	result := callTool(t, handler, map[string]any{"since": "7d"})
	data := unmarshalJSON[logResult](t, result)
	if len(data.Entries) != 1 {
		t.Fatalf("expected 1 entry within the since window, got %d", len(data.Entries))
	}
	if data.Entries[0].Subject != "recent commit" {
		t.Errorf("subject = %q, want %q", data.Entries[0].Subject, "recent commit")
	}
}

func TestLogToolSkipsInvalidCommitInfo(t *testing.T) {
	ts := strconv.FormatInt(time.Now().Add(-1*time.Hour).Unix(), 10)
	worktreeOut := worktreePorcelain(
		struct{ path, branch string }{"/repo", "main"},
		struct{ path, branch string }{pathRepoWorktree + "feature-my-task", branchFeatureMyTask},
		struct{ path, branch string }{"/wt/bugfix-typo", "bugfix/typo"},
	)
	r := logMockRunner(worktreeOut, func(branch string) (string, error) {
		if branch == "bugfix/typo" {
			return "", errors.New("no commits on branch")
		}
		return ts + "\tcommit msg", nil
	})
	hctx := testContext(r)
	handler := handleLog(hctx)

	result := callTool(t, handler, nil)
	data := unmarshalJSON[logResult](t, result)
	if len(data.Entries) != 1 {
		t.Fatalf("expected 1 valid entry (the failing branch excluded), got %d", len(data.Entries))
	}
	if data.Entries[0].Branch != branchFeatureMyTask {
		t.Errorf("branch = %q, want %q", data.Entries[0].Branch, branchFeatureMyTask)
	}
}

func TestLogToolLimit(t *testing.T) {
	ts := strconv.FormatInt(time.Now().Add(-1*time.Hour).Unix(), 10)
	worktreeOut := worktreePorcelain(
		struct{ path, branch string }{"/repo", "main"},
		struct{ path, branch string }{pathRepoWorktree + "feature-my-task", branchFeatureMyTask},
		struct{ path, branch string }{"/wt/bugfix-typo", "bugfix/typo"},
	)
	r := logMockRunner(worktreeOut, func(_ string) (string, error) {
		return ts + "\tcommit msg", nil
	})
	hctx := testContext(r)
	handler := handleLog(hctx)

	result := callTool(t, handler, map[string]any{"limit": 1})
	data := unmarshalJSON[logResult](t, result)
	if len(data.Entries) != 1 {
		t.Errorf("expected 1 entry with limit=1, got %d", len(data.Entries))
	}
}

func TestLogToolDefaultBranchFails(t *testing.T) {
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			return "", errors.New("no default branch")
		},
	}
	hctx := testContext(r)
	handler := handleLog(hctx)

	result := callTool(t, handler, nil)
	errText := resultError(t, result)
	if !strings.Contains(errText, "default branch") {
		t.Errorf("expected default branch error, got: %s", errText)
	}
}

func TestLogToolListWorktreesFails(t *testing.T) {
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			if len(args) > 0 && args[0] == gitSymbolicRef {
				return refsOriginMain, nil
			}
			if len(args) >= 2 && args[0] == gitWorktree && args[1] == gitList {
				return "", errors.New("worktree list failed")
			}
			return "", nil
		},
	}
	hctx := testContext(r)
	handler := handleLog(hctx)

	result := callTool(t, handler, nil)
	errText := resultError(t, result)
	if !strings.Contains(errText, "worktree list failed") {
		t.Errorf("expected worktree list error, got: %s", errText)
	}
}

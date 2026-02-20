package mcp

import (
	"strings"
	"testing"
)

func TestStatusToolEmpty(t *testing.T) {
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			// DefaultBranch
			if len(args) > 0 && args[0] == gitSymbolicRef {
				return refsOriginMain, nil
			}
			// ListWorktrees: just main, no feature branches
			if len(args) > 0 && args[0] == gitWorktree {
				return worktreePorcelain(
					struct{ path, branch string }{"/repo", "main"},
				), nil
			}
			return "", nil
		},
	}
	hctx := testContext(r)
	handler := handleStatus(hctx)

	result := callTool(t, handler, nil)
	data := unmarshalJSON[statusData](t, result)
	if data.Summary.Total != 0 {
		t.Errorf("expected 0 total, got %d", data.Summary.Total)
	}
	if len(data.Worktrees) != 0 {
		t.Errorf("expected 0 worktrees, got %d", len(data.Worktrees))
	}
	if data.StaleDays != 14 {
		t.Errorf("expected stale_days=14, got %d", data.StaleDays)
	}
}

func TestStatusToolWithWorktrees(t *testing.T) {
	porcelain := worktreePorcelain(
		struct{ path, branch string }{"/repo", "main"},
		struct{ path, branch string }{"/wt/feature-login", "feature/login"},
	)

	r := &mockRunner{
		run: func(args ...string) (string, error) {
			if len(args) > 0 && args[0] == gitSymbolicRef {
				return refsOriginMain, nil
			}
			if len(args) > 0 && args[0] == gitWorktree {
				return porcelain, nil
			}
			// LastCommitTime: return a date
			if len(args) > 0 && args[0] == "log" {
				return "2025-01-01T00:00:00Z", nil
			}
			return "", nil
		},
		runInDir: func(dir string, args ...string) (string, error) {
			// status --porcelain: clean
			if len(args) > 0 && args[0] == "status" {
				return "", nil
			}
			// rev-list: no ahead/behind
			if len(args) > 0 && args[0] == "rev-list" {
				return "0\t0", nil
			}
			return "", nil
		},
	}
	hctx := testContext(r)
	handler := handleStatus(hctx)

	result := callTool(t, handler, map[string]any{"stale_days": 7})
	data := unmarshalJSON[statusData](t, result)
	if data.Summary.Total != 1 {
		t.Errorf("expected 1 total, got %d", data.Summary.Total)
	}
	if data.StaleDays != 7 {
		t.Errorf("expected stale_days=7, got %d", data.StaleDays)
	}
}

func TestStatusToolResolvesMainBranch(t *testing.T) {
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			if len(args) > 0 && args[0] == gitWorktree {
				return worktreePorcelain(
					struct{ path, branch string }{"/repo", "main"},
				), nil
			}
			return "", nil
		},
	}

	// Config has DefaultSource — should use that
	hctx := testContext(r)
	hctx.Config.DefaultSource = "develop"
	handler := handleStatus(hctx)

	result := callTool(t, handler, nil)
	if result.IsError {
		t.Fatalf("unexpected error: %s", resultError(t, result))
	}
}

func TestStatusToolNoConfig(t *testing.T) {
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			if len(args) > 0 && args[0] == gitSymbolicRef {
				return refsOriginMain, nil
			}
			if len(args) > 0 && args[0] == gitWorktree {
				return worktreePorcelain(
					struct{ path, branch string }{"/repo", "main"},
				), nil
			}
			return "", nil
		},
	}
	hctx := &HandlerContext{
		Runner:   r,
		Config:   nil,
		RepoRoot: "/repo",
		Version:  "test",
	}
	handler := handleStatus(hctx)

	// Status should work without config (uses git detection)
	result := callTool(t, handler, nil)
	if result.IsError {
		errMsg := resultError(t, result)
		// Only fail if the error is about config — git detection errors are acceptable
		if strings.Contains(errMsg, "not initialized") {
			t.Errorf("status should work without config, got: %s", errMsg)
		}
	}
}

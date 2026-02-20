package mcp

import (
	"strings"
	"testing"
)

func TestListToolEmpty(t *testing.T) {
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			return "", nil // no worktrees
		},
	}
	hctx := testContext(r)
	handler := handleList(hctx)

	result := callTool(t, handler, nil)
	items := unmarshalJSON[[]listItem](t, result)
	if len(items) != 0 {
		t.Errorf("expected 0 items, got %d", len(items))
	}
}

func TestListToolWithWorktrees(t *testing.T) {
	porcelain := worktreePorcelain(
		struct{ path, branch string }{"/repo", "main"},
		struct{ path, branch string }{"/repo/.worktrees/feature-login", "feature/login"},
		struct{ path, branch string }{"/repo/.worktrees/bugfix-typo", "bugfix/typo"},
	)

	r := &mockRunner{
		run: func(args ...string) (string, error) {
			return porcelain, nil
		},
		runInDir: func(dir string, args ...string) (string, error) {
			// status --porcelain returns empty (clean)
			return "", nil
		},
	}
	hctx := testContext(r)
	handler := handleList(hctx)

	result := callTool(t, handler, nil)
	items := unmarshalJSON[[]listItem](t, result)
	if len(items) != 3 {
		t.Fatalf("expected 3 items, got %d", len(items))
	}
}

func TestListToolFilterByType(t *testing.T) {
	porcelain := worktreePorcelain(
		struct{ path, branch string }{"/repo", "main"},
		struct{ path, branch string }{"/repo/.worktrees/feature-login", "feature/login"},
		struct{ path, branch string }{"/repo/.worktrees/bugfix-typo", "bugfix/typo"},
	)

	r := &mockRunner{
		run: func(args ...string) (string, error) {
			return porcelain, nil
		},
		runInDir: func(dir string, args ...string) (string, error) {
			return "", nil
		},
	}
	hctx := testContext(r)
	handler := handleList(hctx)

	result := callTool(t, handler, map[string]any{"type": "bugfix"})
	items := unmarshalJSON[[]listItem](t, result)
	if len(items) != 1 {
		t.Fatalf("expected 1 bugfix item, got %d", len(items))
	}
	if items[0].Type != "bugfix" {
		t.Errorf("type = %q, want bugfix", items[0].Type)
	}
}

func TestListToolInvalidType(t *testing.T) {
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			return "", nil
		},
	}
	hctx := testContext(r)
	handler := handleList(hctx)

	result := callTool(t, handler, map[string]any{"type": "invalid"})
	errText := resultError(t, result)
	if !strings.Contains(errText, "invalid type") {
		t.Errorf("expected 'invalid type' error, got: %s", errText)
	}
}

func TestListToolArchived(t *testing.T) {
	callCount := 0
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			callCount++
			// DefaultBranch call
			if len(args) > 0 && args[0] == gitSymbolicRef {
				return refsOriginMain, nil
			}
			// worktree --porcelain: just main
			if len(args) > 0 && args[0] == gitWorktree {
				return worktreePorcelain(
					struct{ path, branch string }{"/repo", "main"},
				), nil
			}
			// branch --list: includes archived
			if len(args) > 0 && args[0] == "branch" {
				return "  feature/archived-task\n  main\n", nil
			}
			return "", nil
		},
	}
	hctx := testContext(r)
	handler := handleList(hctx)

	result := callTool(t, handler, map[string]any{"archived": true})
	if result.IsError {
		errText := resultError(t, result)
		t.Fatalf("unexpected error: %s", errText)
	}
}

func TestListToolRequiresConfig(t *testing.T) {
	r := &mockRunner{}
	hctx := &HandlerContext{
		Runner:   r,
		Config:   nil, // no config
		RepoRoot: "/repo",
		Version:  "test",
	}
	handler := handleList(hctx)

	// Non-archived list requires config
	result := callTool(t, handler, nil)
	errText := resultError(t, result)
	if !strings.Contains(errText, "not initialized") {
		t.Errorf("expected 'not initialized' error, got: %s", errText)
	}
}

package mcp

import (
	"errors"
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
			if len(args) > 0 && args[0] == gitBranch {
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

func TestListToolDirtyFilter(t *testing.T) {
	porcelain := worktreePorcelain(
		struct{ path, branch string }{"/repo", "main"},
		struct{ path, branch string }{"/repo/.worktrees/feature-a", "feature/a"},
		struct{ path, branch string }{"/repo/.worktrees/feature-b", "feature/b"},
	)
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			return porcelain, nil
		},
		runInDir: func(dir string, args ...string) (string, error) {
			if len(args) > 0 && args[0] == gitStatus {
				if dir == "/repo/.worktrees/feature-a" {
					return dirtyOutput, nil
				}
				return "", nil
			}
			// rev-list for behind/ahead
			if len(args) > 0 && args[0] == gitRevList {
				return "", nil
			}
			return "", nil
		},
	}
	hctx := testContext(r)
	handler := handleList(hctx)

	result := callTool(t, handler, map[string]any{"dirty": true})
	items := unmarshalJSON[[]listItem](t, result)
	if len(items) != 1 {
		t.Fatalf("expected 1 dirty item, got %d", len(items))
	}
	if items[0].Task != "a" {
		t.Errorf("task = %q, want 'a'", items[0].Task)
	}
}

func TestListToolBehindFilter(t *testing.T) {
	porcelain := worktreePorcelain(
		struct{ path, branch string }{"/repo", "main"},
		struct{ path, branch string }{"/repo/.worktrees/feature-a", "feature/a"},
		struct{ path, branch string }{"/repo/.worktrees/feature-b", "feature/b"},
	)
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			return porcelain, nil
		},
		runInDir: func(dir string, args ...string) (string, error) {
			if len(args) > 0 && args[0] == gitStatus {
				return "", nil // clean
			}
			if len(args) > 0 && args[0] == gitRevList {
				if dir == "/repo/.worktrees/feature-b" {
					return "3\t0", nil // 3 behind
				}
				return revListEven, nil
			}
			return "", nil
		},
	}
	hctx := testContext(r)
	handler := handleList(hctx)

	result := callTool(t, handler, map[string]any{"behind": true})
	items := unmarshalJSON[[]listItem](t, result)
	if len(items) != 1 {
		t.Fatalf("expected 1 behind item, got %d", len(items))
	}
	if items[0].Task != "b" {
		t.Errorf("task = %q, want 'b'", items[0].Task)
	}
}

func TestListToolArchivedNoConfig(t *testing.T) {
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
			if len(args) > 0 && args[0] == gitBranch {
				return "  feature/old-task\n", nil
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
	handler := handleList(hctx)

	result := callTool(t, handler, map[string]any{"archived": true})
	if result.IsError {
		t.Fatalf("expected success, got error: %s", resultError(t, result))
	}
}

func TestListToolListError(t *testing.T) {
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			if len(args) > 0 && args[0] == gitWorktree {
				return "", errors.New("git error")
			}
			return "", nil
		},
	}
	hctx := testContext(r)
	handler := handleList(hctx)

	result := callTool(t, handler, nil)
	if !result.IsError {
		t.Error("expected error for list failure")
	}
}

func TestListToolBareEntrySkipped(t *testing.T) {
	// Include a bare entry in the porcelain output
	bare := "worktree /repo\nHEAD abc123\nbranch refs/heads/main\nbare\n\n"
	feature := "worktree /repo/.worktrees/feature-a\nHEAD def456\nbranch refs/heads/feature/a\n\n"
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			return bare + feature, nil
		},
		runInDir: func(dir string, args ...string) (string, error) {
			return "", nil
		},
	}
	hctx := testContext(r)
	handler := handleList(hctx)

	result := callTool(t, handler, nil)
	items := unmarshalJSON[[]listItem](t, result)
	// Only the feature worktree should appear (bare is skipped)
	for _, item := range items {
		if item.Branch == "main" {
			t.Error("bare entry should be skipped")
		}
	}
}

func TestListToolArchivedResolveMainBranchError(t *testing.T) {
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			// Both symbolic-ref and rev-parse fail → can't resolve main branch
			return "", errors.New("no remote")
		},
	}
	hctx := &HandlerContext{
		Runner:   r,
		Config:   nil, // no config
		RepoRoot: "/repo",
		Version:  "test",
	}
	handler := handleList(hctx)

	result := callTool(t, handler, map[string]any{"archived": true})
	if !result.IsError {
		t.Error("expected error when main branch can't be resolved")
	}
}

func TestListToolArchivedListError(t *testing.T) {
	callCount := 0
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			callCount++
			if len(args) > 0 && args[0] == gitSymbolicRef {
				return refsOriginMain, nil
			}
			if len(args) > 0 && args[0] == gitWorktree {
				return worktreePorcelain(
					struct{ path, branch string }{"/repo", "main"},
				), nil
			}
			// branch --list fails
			if len(args) > 0 && args[0] == gitBranch {
				return "", errors.New("branch list error")
			}
			return "", nil
		},
	}
	hctx := testContext(r)
	handler := handleList(hctx)

	result := callTool(t, handler, map[string]any{"archived": true})
	if !result.IsError {
		t.Error("expected error when listing branches fails")
	}
}

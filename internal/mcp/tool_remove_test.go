package mcp

import (
	"errors"
	"strings"
	"testing"
)

func TestRemoveToolRequiresTask(t *testing.T) {
	hctx := testContext(&mockRunner{})
	handler := handleRemove(hctx)

	result := callTool(t, handler, nil)
	errText := resultError(t, result)
	if !strings.Contains(errText, "task is required") {
		t.Errorf("expected 'task is required', got: %s", errText)
	}
}

func TestRemoveToolNotFound(t *testing.T) {
	porcelain := worktreePorcelain(
		struct{ path, branch string }{"/repo", "main"},
	)

	r := &mockRunner{
		run: func(args ...string) (string, error) {
			return porcelain, nil
		},
	}
	hctx := testContext(r)
	handler := handleRemove(hctx)

	result := callTool(t, handler, map[string]any{"task": "nonexistent"})
	errText := resultError(t, result)
	if !strings.Contains(errText, "not found") {
		t.Errorf("expected 'not found' error, got: %s", errText)
	}
}

func TestRemoveToolSuccess(t *testing.T) {
	porcelain := worktreePorcelain(
		struct{ path, branch string }{"/repo", "main"},
		struct{ path, branch string }{"/wt/feature-login", "feature/login"},
	)

	r := &mockRunner{
		run: func(args ...string) (string, error) {
			// ListWorktrees: worktree list --porcelain
			if len(args) >= 2 && args[0] == gitWorktree && args[1] == gitList {
				return porcelain, nil
			}
			// RemoveWorktree: worktree remove
			if len(args) >= 2 && args[0] == gitWorktree && args[1] == gitRemove {
				return "", nil
			}
			// DeleteBranch: branch -D
			if len(args) >= 1 && args[0] == gitBranch {
				return "", nil
			}
			return "", nil
		},
	}
	hctx := testContext(r)
	handler := handleRemove(hctx)

	result := callTool(t, handler, map[string]any{"task": "login"})
	data := unmarshalJSON[removeResult](t, result)
	if data.Task != "login" {
		t.Errorf("task = %q, want %q", data.Task, "login")
	}
	if data.Branch != "feature/login" {
		t.Errorf("branch = %q, want %q", data.Branch, "feature/login")
	}
	if !data.WorktreeRemoved {
		t.Error("expected worktree_removed=true")
	}
	if !data.BranchDeleted {
		t.Error("expected branch_deleted=true")
	}
}

func TestRemoveToolKeepBranch(t *testing.T) {
	porcelain := worktreePorcelain(
		struct{ path, branch string }{"/repo", "main"},
		struct{ path, branch string }{"/wt/feature-login", "feature/login"},
	)

	var branchDeleteCalled bool
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			if len(args) >= 2 && args[0] == gitWorktree && args[1] == gitList {
				return porcelain, nil
			}
			if len(args) >= 2 && args[0] == gitWorktree && args[1] == gitRemove {
				return "", nil
			}
			if len(args) >= 1 && args[0] == gitBranch {
				branchDeleteCalled = true
				return "", nil
			}
			return "", nil
		},
	}
	hctx := testContext(r)
	handler := handleRemove(hctx)

	result := callTool(t, handler, map[string]any{"task": "login", "keep_branch": true})
	data := unmarshalJSON[removeResult](t, result)
	if !data.WorktreeRemoved {
		t.Error("expected worktree_removed=true")
	}
	if data.BranchDeleted {
		t.Error("expected branch_deleted=false when keep_branch=true")
	}
	if branchDeleteCalled {
		t.Error("branch delete should not be called when keep_branch=true")
	}
}

func TestRemoveToolForce(t *testing.T) {
	porcelain := worktreePorcelain(
		struct{ path, branch string }{"/repo", "main"},
		struct{ path, branch string }{"/wt/feature-login", "feature/login"},
	)

	var forceUsed bool
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			if len(args) >= 2 && args[0] == gitWorktree && args[1] == gitList {
				return porcelain, nil
			}
			if len(args) >= 2 && args[0] == gitWorktree && args[1] == gitRemove {
				for _, a := range args {
					if a == "--force" {
						forceUsed = true
					}
				}
				return "", nil
			}
			if len(args) >= 1 && args[0] == gitBranch {
				return "", nil
			}
			return "", nil
		},
	}
	hctx := testContext(r)
	handler := handleRemove(hctx)

	result := callTool(t, handler, map[string]any{"task": "login", "force": true})
	data := unmarshalJSON[removeResult](t, result)
	if !data.WorktreeRemoved {
		t.Error("expected worktree_removed=true")
	}
	if !data.BranchDeleted {
		t.Error("expected branch_deleted=true")
	}
	if !forceUsed {
		t.Error("expected --force flag to be used")
	}
}

func TestRemoveToolRemoveWorktreeFails(t *testing.T) {
	porcelain := worktreePorcelain(
		struct{ path, branch string }{"/repo", "main"},
		struct{ path, branch string }{"/wt/feature-login", "feature/login"},
	)

	r := &mockRunner{
		run: func(args ...string) (string, error) {
			if len(args) >= 2 && args[0] == gitWorktree && args[1] == gitList {
				return porcelain, nil
			}
			if len(args) >= 2 && args[0] == gitWorktree && args[1] == gitRemove {
				return "", errors.New("fatal: worktree has changes, use --force")
			}
			return "", nil
		},
	}
	hctx := testContext(r)
	handler := handleRemove(hctx)

	result := callTool(t, handler, map[string]any{"task": "login"})
	errText := resultError(t, result)
	if !strings.Contains(errText, "has changes") {
		t.Errorf("expected removal error, got: %s", errText)
	}
}

func TestRemoveToolDeleteBranchFails(t *testing.T) {
	porcelain := worktreePorcelain(
		struct{ path, branch string }{"/repo", "main"},
		struct{ path, branch string }{"/wt/feature-login", "feature/login"},
	)

	r := &mockRunner{
		run: func(args ...string) (string, error) {
			if len(args) >= 2 && args[0] == gitWorktree && args[1] == gitList {
				return porcelain, nil
			}
			if len(args) >= 2 && args[0] == gitWorktree && args[1] == gitRemove {
				return "", nil
			}
			if len(args) >= 1 && args[0] == gitBranch {
				return "", errors.New("error: branch not fully merged")
			}
			return "", nil
		},
	}
	hctx := testContext(r)
	handler := handleRemove(hctx)

	// Partial success: worktree removed but branch deletion failed
	result := callTool(t, handler, map[string]any{"task": "login"})
	data := unmarshalJSON[removeResult](t, result)
	if !data.WorktreeRemoved {
		t.Error("expected worktree_removed=true")
	}
	if data.BranchDeleted {
		t.Error("expected branch_deleted=false when deletion fails")
	}
}

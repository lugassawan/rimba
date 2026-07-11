package mcp

import (
	"errors"
	"strings"
	"testing"

	"github.com/lugassawan/rimba/internal/config"
)

func TestMergePlanToolRequiresConfig(t *testing.T) {
	hctx := &HandlerContext{
		Runner:   &mockRunner{},
		Config:   nil,
		RepoRoot: "/repo",
		Version:  "test",
	}
	handler := handleMergePlan(hctx)

	result := callTool(t, handler, nil)
	errText := resultError(t, result)
	if !strings.Contains(errText, "not initialized") {
		t.Errorf("expected config error, got: %s", errText)
	}
}

func TestMergePlanToolNoEligible(t *testing.T) {
	porcelain := worktreePorcelain(
		struct{ path, branch string }{"/repo", "main"},
	)
	r := &mockRunner{
		run: func(args ...string) (string, error) { return porcelain, nil },
	}
	hctx := testContext(r)
	handler := handleMergePlan(hctx)

	result := callTool(t, handler, nil)
	data := unmarshalJSON[mergePlanResult](t, result)
	if len(data.Steps) != 0 {
		t.Errorf("expected 0 steps, got %d", len(data.Steps))
	}
}

func TestMergePlanToolHappyPath(t *testing.T) {
	porcelain := worktreePorcelain(
		struct{ path, branch string }{"/repo", "main"},
		struct{ path, branch string }{"/repo/.worktrees/feature-task-a", "feature/task-a"},
		struct{ path, branch string }{"/repo/.worktrees/feature-task-b", "feature/task-b"},
		struct{ path, branch string }{"/repo/.worktrees/feature-task-c", "feature/task-c"},
	)

	r := &mockRunner{
		run: func(args ...string) (string, error) {
			if len(args) >= 2 && args[0] == gitWorktree && args[1] == gitList {
				return porcelain, nil
			}
			// CollectDiffs: git diff --name-only main...branch
			if len(args) >= 3 && args[0] == gitDiff && args[1] == diffNameOnly {
				ref := args[len(args)-1]
				switch {
				case strings.Contains(ref, "feature/task-a"):
					return "shared.go\nonly-a.go", nil
				case strings.Contains(ref, "feature/task-b"):
					return "shared.go\nonly-b.go", nil
				case strings.Contains(ref, "feature/task-c"):
					return "only-c.go", nil
				}
			}
			return "", nil
		},
	}
	hctx := testContext(r)
	handler := handleMergePlan(hctx)

	result := callTool(t, handler, nil)
	data := unmarshalJSON[mergePlanResult](t, result)

	if len(data.Steps) != 3 {
		t.Fatalf("expected 3 steps, got %d", len(data.Steps))
	}

	// task-c has no conflicts and merges first; task-a and task-b overlap on
	// shared.go, with task-a sorting first alphabetically as the tie-break.
	want := []mergePlanStep{
		{Order: 1, Task: "task-c", Branch: "feature/task-c", Conflicts: 0},
		{Order: 2, Task: "task-a", Branch: "feature/task-a", Conflicts: 1},
		{Order: 3, Task: "task-b", Branch: "feature/task-b", Conflicts: 0},
	}
	for i, w := range want {
		if data.Steps[i] != w {
			t.Errorf("step %d = %+v, want %+v", i, data.Steps[i], w)
		}
	}
}

func TestMergePlanToolListWorktreesFails(t *testing.T) {
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			if len(args) >= 2 && args[0] == gitWorktree && args[1] == gitList {
				return "", errors.New("worktree list failed")
			}
			return "", nil
		},
	}
	hctx := testContext(r)
	handler := handleMergePlan(hctx)

	result := callTool(t, handler, nil)
	errText := resultError(t, result)
	if !strings.Contains(errText, "worktree list failed") {
		t.Errorf("expected worktree list error, got: %s", errText)
	}
}

func TestMergePlanToolCollectDiffsError(t *testing.T) {
	porcelain := worktreePorcelain(
		struct{ path, branch string }{"/repo", "main"},
		struct{ path, branch string }{"/repo/.worktrees/feature-task-a", "feature/task-a"},
	)

	r := &mockRunner{
		run: func(args ...string) (string, error) {
			if len(args) >= 2 && args[0] == gitWorktree && args[1] == gitList {
				return porcelain, nil
			}
			if len(args) >= 3 && args[0] == gitDiff && args[1] == diffNameOnly {
				return "", errors.New("diff failed")
			}
			return "", nil
		},
	}
	hctx := testContext(r)
	handler := handleMergePlan(hctx)

	result := callTool(t, handler, nil)
	errText := resultError(t, result)
	if !strings.Contains(errText, "diff") {
		t.Errorf("expected diff error, got: %s", errText)
	}
}

func TestMergePlanToolRejectsInvalidConfig(t *testing.T) {
	hctx := &HandlerContext{
		Runner:   &mockRunner{},
		Config:   &config.Config{CommandTimeout: "notaduration"},
		RepoRoot: "/repo",
		Version:  "test",
	}
	handler := handleMergePlan(hctx)

	result := callTool(t, handler, nil)
	errText := resultError(t, result)
	if !strings.Contains(errText, "command_timeout") {
		t.Errorf("expected command_timeout validation error, got: %s", errText)
	}
}

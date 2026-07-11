package mcp

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lugassawan/rimba/internal/config"
)

func TestArchiveToolRequiresTask(t *testing.T) {
	hctx := testContext(&mockRunner{})
	handler := handleArchive(hctx)

	result := callTool(t, handler, nil)
	errText := resultError(t, result)
	if !strings.Contains(errText, "task is required") {
		t.Errorf("expected 'task is required', got: %s", errText)
	}
}

func TestArchiveToolRequiresConfig(t *testing.T) {
	hctx := &HandlerContext{
		Runner:   &mockRunner{},
		Config:   nil,
		RepoRoot: "/repo",
		Version:  "test",
	}
	handler := handleArchive(hctx)

	result := callTool(t, handler, map[string]any{"task": "my-task"})
	errText := resultError(t, result)
	if !strings.Contains(errText, "not initialized") {
		t.Errorf("expected config error, got: %s", errText)
	}
}

func TestArchiveToolRejectsInvalidConfig(t *testing.T) {
	hctx := &HandlerContext{
		Runner:   &mockRunner{},
		Config:   &config.Config{CommandTimeout: "notaduration"},
		RepoRoot: "/repo",
		Version:  "test",
	}
	handler := handleArchive(hctx)

	result := callTool(t, handler, map[string]any{"task": "my-task"})
	errText := resultError(t, result)
	if !strings.Contains(errText, "command_timeout") {
		t.Errorf("expected command_timeout validation error, got: %s", errText)
	}
}

func TestArchiveToolNotFound(t *testing.T) {
	porcelain := worktreePorcelain(struct{ path, branch string }{"/repo", "main"})
	r := &mockRunner{
		run: func(args ...string) (string, error) { return porcelain, nil },
	}
	hctx := testContext(r)
	handler := handleArchive(hctx)

	result := callTool(t, handler, map[string]any{"task": "nonexistent"})
	errText := resultError(t, result)
	if !strings.Contains(errText, "not found") {
		t.Errorf("expected 'not found' error, got: %s", errText)
	}
}

func TestArchiveToolHappyPath(t *testing.T) {
	porcelain := worktreePorcelain(
		struct{ path, branch string }{"/repo", "main"},
		struct{ path, branch string }{pathRepoWorktree + "feature-my-task", branchFeatureMyTask},
	)
	var removeArgs []string
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			if len(args) >= 2 && args[0] == gitWorktree && args[1] == gitList {
				return porcelain, nil
			}
			if len(args) >= 2 && args[0] == gitWorktree && args[1] == gitRemove {
				removeArgs = args
				return "", nil
			}
			return "", nil
		},
	}
	hctx := testContext(r)
	handler := handleArchive(hctx)

	result := callTool(t, handler, map[string]any{"task": "my-task"})
	data := unmarshalJSON[archiveResult](t, result)

	if data.Branch != branchFeatureMyTask {
		t.Errorf("branch = %q, want %q", data.Branch, branchFeatureMyTask)
	}
	if data.Path != pathRepoWorktree+"feature-my-task" {
		t.Errorf("path = %q, want %q", data.Path, pathRepoWorktree+"feature-my-task")
	}
	if data.DryRun {
		t.Errorf("expected dry_run=false")
	}
	if len(data.Steps) != 0 {
		t.Errorf("expected no steps in the response for a non-dry-run archive, got %v", data.Steps)
	}
	if removeArgs == nil {
		t.Fatal("expected worktree remove to be called")
	}
	if removeArgs[len(removeArgs)-1] != pathRepoWorktree+"feature-my-task" {
		t.Errorf("expected remove to target the worktree path, got args: %v", removeArgs)
	}
}

func TestArchiveToolForce(t *testing.T) {
	porcelain := worktreePorcelain(
		struct{ path, branch string }{"/repo", "main"},
		struct{ path, branch string }{pathRepoWorktree + "feature-my-task", branchFeatureMyTask},
	)
	var removeArgs []string
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			if len(args) >= 2 && args[0] == gitWorktree && args[1] == gitList {
				return porcelain, nil
			}
			if len(args) >= 2 && args[0] == gitWorktree && args[1] == gitRemove {
				removeArgs = args
				return "", nil
			}
			return "", nil
		},
	}
	hctx := testContext(r)
	handler := handleArchive(hctx)

	callTool(t, handler, map[string]any{"task": "my-task", "force": true})

	found := false
	for _, a := range removeArgs {
		if a == "--force" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected --force in remove args, got: %v", removeArgs)
	}
}

func TestArchiveToolDryRun(t *testing.T) {
	porcelain := worktreePorcelain(
		struct{ path, branch string }{"/repo", "main"},
		struct{ path, branch string }{pathRepoWorktree + "feature-my-task", branchFeatureMyTask},
	)
	var removeCalled bool
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			if len(args) >= 2 && args[0] == gitWorktree && args[1] == gitList {
				return porcelain, nil
			}
			if len(args) >= 2 && args[0] == gitWorktree && args[1] == gitRemove {
				removeCalled = true
				return "", nil
			}
			return "", nil
		},
	}
	hctx := testContext(r)
	handler := handleArchive(hctx)

	result := callTool(t, handler, map[string]any{"task": "my-task", "dry_run": true})
	data := unmarshalJSON[archiveResult](t, result)

	if !data.DryRun {
		t.Errorf("expected dry_run=true")
	}
	if len(data.Steps) == 0 {
		t.Errorf("expected dry-run steps to be populated")
	}
	if removeCalled {
		t.Errorf("worktree remove must not run during a dry run")
	}
}

func TestArchiveToolRemoveFails(t *testing.T) {
	porcelain := worktreePorcelain(
		struct{ path, branch string }{"/repo", "main"},
		struct{ path, branch string }{pathRepoWorktree + "feature-my-task", branchFeatureMyTask},
	)
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			if len(args) >= 2 && args[0] == gitWorktree && args[1] == gitList {
				return porcelain, nil
			}
			if len(args) >= 2 && args[0] == gitWorktree && args[1] == gitRemove {
				return "", errors.New("worktree remove failed")
			}
			return "", nil
		},
	}
	hctx := testContext(r)
	handler := handleArchive(hctx)

	result := callTool(t, handler, map[string]any{"task": "my-task"})
	errText := resultError(t, result)
	if !strings.Contains(errText, "worktree remove failed") {
		t.Errorf("expected remove error, got: %s", errText)
	}
}

func TestArchiveToolServiceScoped(t *testing.T) {
	tmpDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(tmpDir, "auth-api"), 0o755); err != nil {
		t.Fatal(err)
	}

	porcelain := worktreePorcelain(
		struct{ path, branch string }{tmpDir, "main"},
		struct{ path, branch string }{tmpDir + "/.worktrees/auth-api-feature-my-task", branchServiceFeatureMyTask},
	)
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			if len(args) >= 2 && args[0] == gitWorktree && args[1] == gitList {
				return porcelain, nil
			}
			return "", nil
		},
	}
	hctx := &HandlerContext{
		Runner:   r,
		Config:   testConfig(),
		RepoRoot: tmpDir,
		Version:  "test",
	}
	handler := handleArchive(hctx)

	result := callTool(t, handler, map[string]any{"task": "auth-api/my-task"})
	data := unmarshalJSON[archiveResult](t, result)

	if data.Branch != branchServiceFeatureMyTask {
		t.Errorf("branch = %q, want %q", data.Branch, branchServiceFeatureMyTask)
	}
}

// TestArchiveToolCustomPrefixCtxInjection locks in the fix for #388:
// without injecting cfg into ctx, operations.FindWorktree's
// config.PrefixSetFromContext(ctx) falls back to built-in prefixes and a
// "custom/my-task" worktree would never be found for task "my-task".
func TestArchiveToolCustomPrefixCtxInjection(t *testing.T) {
	porcelain := worktreePorcelain(
		struct{ path, branch string }{"/repo", "main"},
		struct{ path, branch string }{pathRepoWorktree + "custom-my-task", "custom/my-task"},
	)
	var removeArgs []string
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			if len(args) >= 2 && args[0] == gitWorktree && args[1] == gitList {
				return porcelain, nil
			}
			if len(args) >= 2 && args[0] == gitWorktree && args[1] == gitRemove {
				removeArgs = args
				return "", nil
			}
			return "", nil
		},
	}
	hctx := &HandlerContext{
		Runner: r,
		Config: &config.Config{
			WorktreeDir:   ".worktrees",
			DefaultSource: "main",
			Resolver: &config.ResolverConfig{
				Prefix: []config.PrefixEntry{{Prefix: "custom/"}},
			},
		},
		RepoRoot: "/repo",
		Version:  "test",
	}
	handler := handleArchive(hctx)

	result := callTool(t, handler, map[string]any{"task": "my-task"})
	data := unmarshalJSON[archiveResult](t, result)

	if data.Branch != "custom/my-task" {
		t.Errorf("branch = %q, want %q (custom prefix must resolve)", data.Branch, "custom/my-task")
	}
	if removeArgs == nil {
		t.Fatal("expected worktree remove to be called")
	}
}

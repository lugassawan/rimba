package mcp

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lugassawan/rimba/internal/config"
	"github.com/lugassawan/rimba/internal/trust"
)

const branchFeatureRestoredTask = "feature/restored-task"

func TestRestoreToolRequiresTask(t *testing.T) {
	hctx := testContext(&mockRunner{})
	handler := handleRestore(hctx)

	result := callTool(t, handler, nil)
	errText := resultError(t, result)
	if !strings.Contains(errText, "task is required") {
		t.Errorf("expected 'task is required', got: %s", errText)
	}
}

func TestRestoreToolRequiresConfig(t *testing.T) {
	hctx := &HandlerContext{
		Runner:   &mockRunner{},
		Config:   nil,
		RepoRoot: "/repo",
		Version:  "test",
	}
	handler := handleRestore(hctx)

	result := callTool(t, handler, map[string]any{"task": "my-task"})
	errText := resultError(t, result)
	if !strings.Contains(errText, "not initialized") {
		t.Errorf("expected config error, got: %s", errText)
	}
}

func TestRestoreToolRejectsInvalidConfig(t *testing.T) {
	hctx := &HandlerContext{
		Runner:   &mockRunner{},
		Config:   &config.Config{CommandTimeout: "notaduration"},
		RepoRoot: "/repo",
		Version:  "test",
	}
	handler := handleRestore(hctx)

	result := callTool(t, handler, map[string]any{"task": "my-task"})
	errText := resultError(t, result)
	if !strings.Contains(errText, "command_timeout") {
		t.Errorf("expected command_timeout validation error, got: %s", errText)
	}
}

func TestRestoreToolNotFound(t *testing.T) {
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			if len(args) > 0 && args[0] == gitBranch {
				return "main", nil
			}
			if len(args) >= 2 && args[0] == gitWorktree && args[1] == gitList {
				return worktreePorcelain(struct{ path, branch string }{"/repo", "main"}), nil
			}
			return "", nil
		},
	}
	hctx := testContext(r)
	handler := handleRestore(hctx)

	result := callTool(t, handler, map[string]any{"task": "nonexistent"})
	errText := resultError(t, result)
	if !strings.Contains(errText, "no archived branch found") {
		t.Errorf("expected 'no archived branch found' error, got: %s", errText)
	}
}

// restoreHappyPathRunner builds a mock runner for a plain restore: one
// archived branch ("feature/restored-task") not attached to any active
// worktree (only "main" is active).
func restoreHappyPathRunner() *mockRunner {
	return &mockRunner{
		run: func(args ...string) (string, error) {
			switch {
			case len(args) > 0 && args[0] == gitBranch:
				return "main\n" + branchFeatureRestoredTask, nil
			case len(args) >= 2 && args[0] == gitWorktree && args[1] == gitList:
				return worktreePorcelain(struct{ path, branch string }{"/repo", "main"}), nil
			case len(args) >= 2 && args[0] == gitWorktree && args[1] == gitWorktreeAdd:
				return "", nil
			}
			return "", nil
		},
	}
}

func TestRestoreToolHappyPath(t *testing.T) {
	r := restoreHappyPathRunner()
	hctx := testContext(r)
	handler := handleRestore(hctx)

	result := callTool(t, handler, map[string]any{
		"task": "restored-task", "skip_deps": true, "skip_hooks": true,
	})
	data := unmarshalJSON[restoreResult](t, result)

	if data.Branch != branchFeatureRestoredTask {
		t.Errorf("branch = %q, want %q", data.Branch, branchFeatureRestoredTask)
	}
	if data.Task != "restored-task" {
		t.Errorf("task = %q, want %q", data.Task, "restored-task")
	}
	if !strings.Contains(data.Path, ".worktrees") {
		t.Errorf("path = %q, expected to contain worktree dir", data.Path)
	}
}

func TestRestoreToolAddWorktreeFails(t *testing.T) {
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			switch {
			case len(args) > 0 && args[0] == gitBranch:
				return "main\n" + branchFeatureRestoredTask, nil
			case len(args) >= 2 && args[0] == gitWorktree && args[1] == gitList:
				return worktreePorcelain(struct{ path, branch string }{"/repo", "main"}), nil
			case len(args) >= 2 && args[0] == gitWorktree && args[1] == gitWorktreeAdd:
				return "", errors.New("worktree add failed")
			}
			return "", nil
		},
	}
	hctx := testContext(r)
	handler := handleRestore(hctx)

	result := callTool(t, handler, map[string]any{
		"task": "restored-task", "skip_deps": true, "skip_hooks": true,
	})
	errText := resultError(t, result)
	if !strings.Contains(errText, "worktree add failed") {
		t.Errorf("expected worktree add error, got: %s", errText)
	}
}

func TestRestoreToolTrustGateUntrusted(t *testing.T) {
	tmpDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmpDir, ".gitignore"), nil, 0644); err != nil {
		t.Fatal(err)
	}
	cfg := testConfig()
	cfg.PostCreate = []string{"make install"}
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			switch {
			case len(args) > 0 && args[0] == gitBranch:
				return "main\n" + branchFeatureRestoredTask, nil
			case len(args) >= 2 && args[0] == gitWorktree && args[1] == gitList:
				return worktreePorcelain(struct{ path, branch string }{tmpDir, "main"}), nil
			}
			return "", nil
		},
	}
	hctx := &HandlerContext{
		Runner:   r,
		Config:   cfg,
		RepoRoot: tmpDir,
		Version:  "test",
	}
	handler := handleRestore(hctx)

	result := callTool(t, handler, map[string]any{"task": "restored-task"})
	errText := resultError(t, result)
	if !strings.Contains(errText, "rimba trust") {
		t.Errorf("untrusted error should mention 'rimba trust', got: %s", errText)
	}
}

func TestRestoreToolTrustGatePreTrusted(t *testing.T) {
	tmpDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmpDir, ".gitignore"), nil, 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(tmpDir, ".rimba"), 0750); err != nil {
		t.Fatal(err)
	}

	cfg := testConfig()
	cfg.PostCreate = []string{"echo trusted"}
	cfg.CopyFiles = nil

	h := trust.Hash(cfg)
	if err := trust.Record(tmpDir, h); err != nil {
		t.Fatal(err)
	}

	r := &mockRunner{
		run: func(args ...string) (string, error) {
			switch {
			case len(args) > 0 && args[0] == gitBranch:
				return "main\n" + branchFeatureRestoredTask, nil
			case len(args) >= 2 && args[0] == gitWorktree && args[1] == gitList:
				return worktreePorcelain(struct{ path, branch string }{tmpDir, "main"}), nil
			case len(args) >= 2 && args[0] == gitWorktree && args[1] == gitWorktreeAdd:
				return "", nil
			}
			return "", nil
		},
	}
	hctx := &HandlerContext{
		Runner:   r,
		Config:   cfg,
		RepoRoot: tmpDir,
		Version:  "test",
	}
	handler := handleRestore(hctx)

	result := callTool(t, handler, map[string]any{"task": "restored-task", "skip_deps": true})
	data := unmarshalJSON[restoreResult](t, result)
	if data.Task != "restored-task" {
		t.Errorf("pre-trusted restore should succeed, task = %q", data.Task)
	}
}

func TestRestoreToolWithDepsModules(t *testing.T) {
	t.Setenv("RIMBA_TRUST_YES", "1")
	tmpDir := t.TempDir()
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			switch {
			case len(args) > 0 && args[0] == gitBranch:
				return "main\n" + branchFeatureRestoredTask, nil
			case len(args) >= 2 && args[0] == gitWorktree && args[1] == gitList:
				return worktreePorcelain(struct{ path, branch string }{tmpDir, "main"}), nil
			case len(args) >= 2 && args[0] == gitWorktree && args[1] == gitWorktreeAdd:
				return "", nil
			}
			return "", nil
		},
	}
	cfg := testConfig()
	cfg.Deps = &config.DepsConfig{Modules: []config.ModuleConfig{{Dir: ".", Lockfile: "go.sum", Install: "go mod download"}}}
	hctx := &HandlerContext{
		Runner:   r,
		Config:   cfg,
		RepoRoot: tmpDir,
		Version:  "test",
	}
	handler := handleRestore(hctx)

	result := callTool(t, handler, map[string]any{
		"task": "restored-task", "skip_deps": true, "skip_hooks": true,
	})
	data := unmarshalJSON[restoreResult](t, result)
	if data.Branch != branchFeatureRestoredTask {
		t.Errorf("branch = %q, want %q", data.Branch, branchFeatureRestoredTask)
	}
}

func TestRestoreToolPostCreateSetupFails(t *testing.T) {
	var worktreeListCalls int
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			switch {
			case len(args) > 0 && args[0] == gitBranch:
				return "main\n" + branchFeatureRestoredTask, nil
			case len(args) >= 2 && args[0] == gitWorktree && args[1] == gitList:
				worktreeListCalls++
				if worktreeListCalls > 1 {
					// Second call comes from PostCreateSetup's dependency install.
					return "", errors.New("worktree list failed")
				}
				return worktreePorcelain(struct{ path, branch string }{"/repo", "main"}), nil
			case len(args) >= 2 && args[0] == gitWorktree && args[1] == gitWorktreeAdd:
				return "", nil
			}
			return "", nil
		},
	}
	hctx := testContext(r)
	handler := handleRestore(hctx)

	result := callTool(t, handler, map[string]any{"task": "restored-task", "skip_hooks": true})
	errText := resultError(t, result)
	if !strings.Contains(errText, "worktree list failed") {
		t.Errorf("expected post-create setup error, got: %s", errText)
	}
}

// TestRestoreToolCustomPrefixCtxInjection locks in the fix for #388: without
// injecting cfg into ctx, FindArchivedBranch's config.PrefixSetFromContext(ctx)
// falls back to built-in prefixes and "custom/archived-task" would never be
// recognized as an archived branch for task "archived-task".
func TestRestoreToolCustomPrefixCtxInjection(t *testing.T) {
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			switch {
			case len(args) > 0 && args[0] == gitBranch:
				return "main\ncustom/archived-task", nil
			case len(args) >= 2 && args[0] == gitWorktree && args[1] == gitList:
				return worktreePorcelain(struct{ path, branch string }{"/repo", "main"}), nil
			case len(args) >= 2 && args[0] == gitWorktree && args[1] == gitWorktreeAdd:
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
	handler := handleRestore(hctx)

	result := callTool(t, handler, map[string]any{
		"task": "archived-task", "skip_deps": true, "skip_hooks": true,
	})
	data := unmarshalJSON[restoreResult](t, result)

	if data.Branch != "custom/archived-task" {
		t.Errorf("branch = %q, want %q (custom prefix must resolve)", data.Branch, "custom/archived-task")
	}
}

func TestRestoreToolServiceScoped(t *testing.T) {
	tmpDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(tmpDir, "auth-api"), 0o755); err != nil {
		t.Fatal(err)
	}

	r := &mockRunner{
		run: func(args ...string) (string, error) {
			switch {
			case len(args) > 0 && args[0] == gitBranch:
				return "main\n" + branchServiceFeatureMyTask, nil
			case len(args) >= 2 && args[0] == gitWorktree && args[1] == gitList:
				return worktreePorcelain(struct{ path, branch string }{tmpDir, "main"}), nil
			case len(args) >= 2 && args[0] == gitWorktree && args[1] == gitWorktreeAdd:
				return "", nil
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
	handler := handleRestore(hctx)

	result := callTool(t, handler, map[string]any{
		"task": "auth-api/my-task", "skip_deps": true, "skip_hooks": true,
	})
	data := unmarshalJSON[restoreResult](t, result)

	if data.Branch != branchServiceFeatureMyTask {
		t.Errorf("branch = %q, want %q", data.Branch, branchServiceFeatureMyTask)
	}
}

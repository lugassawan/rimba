package mcp

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lugassawan/rimba/internal/config"
)

const (
	remoteURLStub    = "https://example.com/repo.git"
	pathRepoWorktree = "/repo/.worktrees/"
)

func TestRenameToolRequiresTask(t *testing.T) {
	hctx := testContext(&mockRunner{})
	handler := handleRename(hctx)

	result := callTool(t, handler, nil)
	errText := resultError(t, result)
	if !strings.Contains(errText, "task is required") {
		t.Errorf("expected 'task is required', got: %s", errText)
	}
}

func TestRenameToolRequiresConfig(t *testing.T) {
	hctx := &HandlerContext{
		Runner:   &mockRunner{},
		Config:   nil,
		RepoRoot: "/repo",
		Version:  "test",
	}
	handler := handleRename(hctx)

	result := callTool(t, handler, map[string]any{"task": "my-task"})
	errText := resultError(t, result)
	if !strings.Contains(errText, "not initialized") {
		t.Errorf("expected config error, got: %s", errText)
	}
}

func TestRenameToolRejectsInvalidConfig(t *testing.T) {
	hctx := &HandlerContext{
		Runner:   &mockRunner{},
		Config:   &config.Config{CommandTimeout: "notaduration"},
		RepoRoot: "/repo",
		Version:  "test",
	}
	handler := handleRename(hctx)

	result := callTool(t, handler, map[string]any{"task": "my-task"})
	errText := resultError(t, result)
	if !strings.Contains(errText, "command_timeout") {
		t.Errorf("expected command_timeout validation error, got: %s", errText)
	}
}

func TestRenameToolNotFound(t *testing.T) {
	porcelain := worktreePorcelain(
		struct{ path, branch string }{"/repo", "main"},
	)
	r := &mockRunner{
		run: func(args ...string) (string, error) { return porcelain, nil },
	}
	hctx := testContext(r)
	handler := handleRename(hctx)

	result := callTool(t, handler, map[string]any{"task": "nonexistent"})
	errText := resultError(t, result)
	if !strings.Contains(errText, "not found") {
		t.Errorf("expected 'not found' error, got: %s", errText)
	}
}

func TestRenameToolOrphanedHardErrors(t *testing.T) {
	porcelain := worktreePorcelain(
		struct{ path, branch string }{"/repo", "main"},
		struct{ path, branch string }{"/wt/proj-123", "PROJ-123"},
	)
	r := &mockRunner{
		run: func(args ...string) (string, error) { return porcelain, nil },
	}
	hctx := &HandlerContext{
		Runner: r,
		Config: &config.Config{
			DefaultSource: "main",
			Resolver: &config.ResolverConfig{
				Prefix: []config.PrefixEntry{{Prefix: "TASK-"}},
			},
		},
		RepoRoot: "/repo",
		Version:  "test",
	}
	handler := handleRename(hctx)

	result := callTool(t, handler, map[string]any{"task": "PROJ-123", "new_task": "PROJ-456"})
	errText := resultError(t, result)
	if !strings.Contains(errText, "re-add the prefix") {
		t.Errorf("expected orphan-guard error mentioning re-adding the prefix, got: %s", errText)
	}
}

// renameHappyPathRunner builds a mock runner for a plain rename (no push/deps/hooks).
func renameHappyPathRunner(porcelain string) *mockRunner {
	return &mockRunner{
		run: func(args ...string) (string, error) {
			if len(args) >= 2 && args[0] == gitWorktree && args[1] == gitList {
				return porcelain, nil
			}
			if len(args) > 0 && args[0] == gitRevParse {
				return "", errors.New("not found") // BranchExists returns false
			}
			return "", nil
		},
	}
}

func TestRenameToolHappyPath(t *testing.T) {
	porcelain := worktreePorcelain(
		struct{ path, branch string }{"/repo", "main"},
		struct{ path, branch string }{pathRepoWorktree + "feature-my-task", branchFeatureMyTask},
	)
	r := renameHappyPathRunner(porcelain)
	hctx := testContext(r)
	handler := handleRename(hctx)

	result := callTool(t, handler, map[string]any{
		"task": "my-task", "new_task": "auth", "skip_deps": true, "skip_hooks": true,
	})
	data := unmarshalJSON[renameResult](t, result)

	if data.OldBranch != branchFeatureMyTask {
		t.Errorf("old_branch = %q, want %q", data.OldBranch, branchFeatureMyTask)
	}
	if data.NewBranch != "feature/auth" {
		t.Errorf("new_branch = %q, want %q", data.NewBranch, "feature/auth")
	}
	if data.OldPath != pathRepoWorktree+"feature-my-task" {
		t.Errorf("old_path = %q, want %q", data.OldPath, pathRepoWorktree+"feature-my-task")
	}
	if data.NewPath != pathRepoWorktree+"feature-auth" {
		t.Errorf("new_path = %q, want %q", data.NewPath, pathRepoWorktree+"feature-auth")
	}
}

func TestRenameToolRetypeOnlyDefaultsNewTask(t *testing.T) {
	porcelain := worktreePorcelain(
		struct{ path, branch string }{"/repo", "main"},
		struct{ path, branch string }{pathRepoWorktree + "feature-my-task", branchFeatureMyTask},
	)
	r := renameHappyPathRunner(porcelain)
	hctx := testContext(r)
	handler := handleRename(hctx)

	result := callTool(t, handler, map[string]any{
		"task": "my-task", "type": "bugfix", "skip_deps": true, "skip_hooks": true,
	})
	data := unmarshalJSON[renameResult](t, result)

	if data.NewBranch != "bugfix/my-task" {
		t.Errorf("new_branch = %q, want %q", data.NewBranch, "bugfix/my-task")
	}
}

func TestRenameToolInvalidType(t *testing.T) {
	porcelain := worktreePorcelain(
		struct{ path, branch string }{"/repo", "main"},
		struct{ path, branch string }{pathRepoWorktree + "feature-my-task", branchFeatureMyTask},
	)
	r := renameHappyPathRunner(porcelain)
	hctx := testContext(r)
	handler := handleRename(hctx)

	result := callTool(t, handler, map[string]any{"task": "my-task", "type": "bogus"})
	errText := resultError(t, result)
	if !strings.Contains(errText, "invalid type") {
		t.Errorf("expected 'invalid type' error, got: %s", errText)
	}
}

func TestRenameToolBranchAlreadyExists(t *testing.T) {
	porcelain := worktreePorcelain(
		struct{ path, branch string }{"/repo", "main"},
		struct{ path, branch string }{pathRepoWorktree + "feature-my-task", branchFeatureMyTask},
	)
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			if len(args) >= 2 && args[0] == gitWorktree && args[1] == gitList {
				return porcelain, nil
			}
			if len(args) > 0 && args[0] == gitRevParse {
				return "abc123", nil // BranchExists returns true
			}
			return "", nil
		},
	}
	hctx := testContext(r)
	handler := handleRename(hctx)

	result := callTool(t, handler, map[string]any{"task": "my-task", "new_task": "auth"})
	errText := resultError(t, result)
	if !strings.Contains(errText, "already exists") {
		t.Errorf("expected 'already exists' error, got: %s", errText)
	}
}

// renamePushMockOpts controls the git responses relevant to --push's
// publish/delete steps.
type renamePushMockOpts struct {
	remoteExists bool
	hasUpstream  bool
	pushErr      error
	deleteErr    error
}

func renamePushRunner(porcelain string, opts renamePushMockOpts) *mockRunner {
	return &mockRunner{
		run:      renamePushRunFn(porcelain, opts),
		runInDir: renamePushRunInDirFn(opts),
	}
}

func renamePushRunFn(porcelain string, opts renamePushMockOpts) func(args ...string) (string, error) {
	return func(args ...string) (string, error) {
		if len(args) >= 2 && args[0] == gitWorktree && args[1] == gitList {
			return porcelain, nil
		}
		if len(args) >= 2 && args[0] == "remote" && args[1] == "get-url" {
			if opts.remoteExists {
				return remoteURLStub, nil
			}
			return "", errors.New("not found")
		}
		if len(args) > 0 && args[0] == gitRevParse {
			return "", errors.New("not found") // BranchExists returns false
		}
		if len(args) > 0 && args[0] == pushCmd {
			return "", opts.deleteErr // push origin --delete <old>
		}
		return "", nil
	}
}

func renamePushRunInDirFn(opts renamePushMockOpts) func(dir string, args ...string) (string, error) {
	return func(_ string, args ...string) (string, error) {
		if len(args) > 0 && args[0] == gitRevParse {
			if opts.hasUpstream {
				return "origin/" + branchFeatureMyTask, nil
			}
			return "", errors.New("not found")
		}
		if len(args) > 0 && args[0] == pushCmd {
			return "", opts.pushErr // push -u origin <new>
		}
		return "", nil
	}
}

func TestRenameToolPushHappyPath(t *testing.T) {
	porcelain := worktreePorcelain(
		struct{ path, branch string }{"/repo", "main"},
		struct{ path, branch string }{pathRepoWorktree + "feature-my-task", branchFeatureMyTask},
	)
	r := renamePushRunner(porcelain, renamePushMockOpts{remoteExists: true, hasUpstream: true})
	hctx := testContext(r)
	handler := handleRename(hctx)

	result := callTool(t, handler, map[string]any{
		"task": "my-task", "new_task": "auth", "push": true, "skip_deps": true, "skip_hooks": true,
	})
	data := unmarshalJSON[renameResult](t, result)

	if !data.Published {
		t.Errorf("expected published=true")
	}
	if !data.RemoteDeleted {
		t.Errorf("expected remote_deleted=true")
	}
	if data.PublishError != "" {
		t.Errorf("expected no publish_error, got: %s", data.PublishError)
	}
}

func TestRenameToolPushPublishFailure(t *testing.T) {
	porcelain := worktreePorcelain(
		struct{ path, branch string }{"/repo", "main"},
		struct{ path, branch string }{pathRepoWorktree + "feature-my-task", branchFeatureMyTask},
	)
	r := renamePushRunner(porcelain, renamePushMockOpts{
		remoteExists: true, hasUpstream: true, pushErr: errors.New("push failed"),
	})
	hctx := testContext(r)
	handler := handleRename(hctx)

	result := callTool(t, handler, map[string]any{
		"task": "my-task", "new_task": "auth", "push": true, "skip_deps": true, "skip_hooks": true,
	})
	data := unmarshalJSON[renameResult](t, result)

	if data.Published {
		t.Errorf("expected published=false when push fails")
	}
	if !strings.Contains(data.PublishError, "push failed") {
		t.Errorf("expected publish_error to mention 'push failed', got: %s", data.PublishError)
	}
	if data.RemoteDeleted {
		t.Errorf("expected remote_deleted=false when publish failed (old branch must survive)")
	}
}

func TestRenameToolTrustGateUntrusted(t *testing.T) {
	tmpDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmpDir, ".gitignore"), nil, 0644); err != nil {
		t.Fatal(err)
	}
	porcelain := worktreePorcelain(
		struct{ path, branch string }{tmpDir, "main"},
		struct{ path, branch string }{filepath.Join(tmpDir, ".worktrees", "feature-my-task"), branchFeatureMyTask},
	)
	cfg := testConfig()
	cfg.PostRename = []string{"make install"}
	hctx := &HandlerContext{
		Runner:   renameHappyPathRunner(porcelain),
		Config:   cfg,
		RepoRoot: tmpDir,
		Version:  "test",
	}
	handler := handleRename(hctx)

	result := callTool(t, handler, map[string]any{"task": "my-task", "new_task": "auth"})
	errText := resultError(t, result)
	if !strings.Contains(errText, "rimba trust") {
		t.Errorf("untrusted error should mention 'rimba trust', got: %s", errText)
	}
}

func TestRenameToolWithDepsModules(t *testing.T) {
	t.Setenv("RIMBA_TRUST_YES", "1")
	tmpDir := t.TempDir()
	porcelain := worktreePorcelain(
		struct{ path, branch string }{tmpDir, "main"},
		struct{ path, branch string }{filepath.Join(tmpDir, ".worktrees", "feature-my-task"), branchFeatureMyTask},
	)
	r := renameHappyPathRunner(porcelain)
	cfg := testConfig()
	cfg.Deps = &config.DepsConfig{Modules: []config.ModuleConfig{{Dir: ".", Lockfile: "go.sum", Install: "go mod download"}}}
	hctx := &HandlerContext{
		Runner:   r,
		Config:   cfg,
		RepoRoot: tmpDir,
		Version:  "test",
	}
	handler := handleRename(hctx)

	result := callTool(t, handler, map[string]any{
		"task": "my-task", "new_task": "auth", "skip_deps": true, "skip_hooks": true,
	})
	data := unmarshalJSON[renameResult](t, result)
	if data.NewBranch != "feature/auth" {
		t.Errorf("new_branch = %q, want %q", data.NewBranch, "feature/auth")
	}
}

func TestRenameToolPostRenameSetupFails(t *testing.T) {
	porcelain := worktreePorcelain(
		struct{ path, branch string }{"/repo", "main"},
		struct{ path, branch string }{pathRepoWorktree + "feature-my-task", branchFeatureMyTask},
	)
	var worktreeListCalls int
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			if len(args) >= 2 && args[0] == gitWorktree && args[1] == gitList {
				worktreeListCalls++
				if worktreeListCalls > 1 {
					// Second call comes from PostRenameSetup's dependency refresh.
					return "", errors.New("worktree list failed")
				}
				return porcelain, nil
			}
			if len(args) > 0 && args[0] == gitRevParse {
				return "", errors.New("not found") // BranchExists returns false
			}
			return "", nil
		},
	}
	hctx := testContext(r)
	handler := handleRename(hctx)

	result := callTool(t, handler, map[string]any{
		"task": "my-task", "new_task": "auth", "skip_hooks": true,
	})
	errText := resultError(t, result)
	if !strings.Contains(errText, "worktree list failed") {
		t.Errorf("expected post-rename setup error, got: %s", errText)
	}
}

// TestRenameToolCustomPrefixCtxInjection locks in #388: without ctx injection,
// "custom/" is unrecognized and the rename silently falls back to "feature/".
func TestRenameToolCustomPrefixCtxInjection(t *testing.T) {
	porcelain := worktreePorcelain(
		struct{ path, branch string }{"/repo", "main"},
		struct{ path, branch string }{pathRepoWorktree + "custom-my-task", "custom/my-task"},
	)
	r := renameHappyPathRunner(porcelain)
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
	handler := handleRename(hctx)

	result := callTool(t, handler, map[string]any{
		"task": "my-task", "new_task": "my-task-v2", "skip_deps": true, "skip_hooks": true,
	})
	data := unmarshalJSON[renameResult](t, result)

	if data.NewBranch != "custom/my-task-v2" {
		t.Errorf("new_branch = %q, want %q (custom prefix must survive rename)", data.NewBranch, "custom/my-task-v2")
	}
}

func TestRenameToolServiceScoped(t *testing.T) {
	tmpDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(tmpDir, "auth-api"), 0o755); err != nil {
		t.Fatal(err)
	}

	porcelain := worktreePorcelain(
		struct{ path, branch string }{tmpDir, "main"},
		struct{ path, branch string }{tmpDir + "/.worktrees/auth-api-feature-my-task", branchServiceFeatureMyTask},
	)
	r := renameHappyPathRunner(porcelain)
	hctx := &HandlerContext{
		Runner:   r,
		Config:   testConfig(),
		RepoRoot: tmpDir,
		Version:  "test",
	}
	handler := handleRename(hctx)

	result := callTool(t, handler, map[string]any{
		"task": "auth-api/my-task", "new_task": "my-task-v2", "skip_deps": true, "skip_hooks": true,
	})
	data := unmarshalJSON[renameResult](t, result)

	if data.NewBranch != "auth-api/feature/my-task-v2" {
		t.Errorf("new_branch = %q, want %q", data.NewBranch, "auth-api/feature/my-task-v2")
	}
}

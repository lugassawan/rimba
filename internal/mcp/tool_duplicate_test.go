package mcp

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lugassawan/rimba/internal/config"
)

func TestDuplicateToolSchema(t *testing.T) {
	tools := NewServer(testContext(&mockRunner{})).ListTools()
	tool, ok := tools["duplicate"]
	if !ok {
		t.Fatal("duplicate tool is not registered")
	}
	if len(tool.Tool.InputSchema.Properties) != 5 {
		t.Fatalf("properties = %d, want 5", len(tool.Tool.InputSchema.Properties))
	}
	if len(tool.Tool.InputSchema.Required) != 1 || tool.Tool.InputSchema.Required[0] != "task" {
		t.Errorf("required = %v, want [task]", tool.Tool.InputSchema.Required)
	}
	for _, name := range []string{"task", "as", "skip_deps", "skip_hooks", "dry_run"} {
		if _, ok := tool.Tool.InputSchema.Properties[name]; !ok {
			t.Errorf("missing schema property %q", name)
		}
	}
}

func TestDuplicateToolRequiresTask(t *testing.T) {
	result := callTool(t, handleDuplicate(testContext(&mockRunner{})), nil)
	if got := resultError(t, result); !strings.Contains(got, "task is required") {
		t.Errorf("error = %q, want missing-task guidance", got)
	}
}

func TestDuplicateToolRequiresConfig(t *testing.T) {
	hctx := &HandlerContext{Runner: &mockRunner{}, RepoRoot: "/repo", Version: "test"}
	result := callTool(t, handleDuplicate(hctx), map[string]any{"task": "my-task"})
	if got := resultError(t, result); !strings.Contains(got, "not initialized") {
		t.Errorf("error = %q, want config guidance", got)
	}
}

func TestDuplicateToolDryRunSkipsTrustAndMutation(t *testing.T) {
	repoRoot := t.TempDir()
	if err := os.WriteFile(filepath.Join(repoRoot, ".gitignore"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	created := false
	r := duplicateToolRunner(repoRoot, "feature/my-task", &created, false)
	cfg := testConfig()
	cfg.PostCreate = []string{"make install"}
	hctx := &HandlerContext{Runner: r, Config: cfg, RepoRoot: repoRoot, Version: "test"}

	result := callTool(t, handleDuplicate(hctx), map[string]any{"task": "my-task", "dry_run": true})
	data := unmarshalJSON[duplicateResult](t, result)
	if !data.DryRun || data.Branch != "feature/my-task-1" {
		t.Errorf("result = %+v, want dry-run feature/my-task-1", data)
	}
	if created {
		t.Fatal("dry-run must not create a worktree")
	}
}

func TestDuplicateToolSuccess(t *testing.T) {
	repoRoot := t.TempDir()
	created := false
	r := duplicateToolRunner(repoRoot, "feature/my-task", &created, false)
	hctx := &HandlerContext{Runner: r, Config: testConfig(), RepoRoot: repoRoot, Version: "test"}

	result := callTool(t, handleDuplicate(hctx), map[string]any{
		"task": "my-task", "as": "copy", "skip_deps": true, "skip_hooks": true,
	})
	data := unmarshalJSON[duplicateResult](t, result)
	if data.SourceTask != "my-task" || data.SourceBranch != "feature/my-task" {
		t.Errorf("source = %+v, want source task and branch", data)
	}
	if data.Task != "copy" || data.Branch != "feature/copy" {
		t.Errorf("destination = %+v, want feature/copy", data)
	}
	if !created {
		t.Fatal("expected worktree creation")
	}
}

func TestDuplicateToolCustomPrefix(t *testing.T) {
	repoRoot := t.TempDir()
	created := false
	r := duplicateToolRunner(repoRoot, "custom/my-task", &created, false)
	hctx := &HandlerContext{
		Runner: r,
		Config: &config.Config{
			WorktreeDir:   ".worktrees",
			DefaultSource: "main",
			Resolver:      &config.ResolverConfig{Prefix: []config.PrefixEntry{{Prefix: "custom/"}}},
		},
		RepoRoot: repoRoot,
		Version:  "test",
	}

	result := callTool(t, handleDuplicate(hctx), map[string]any{
		"task": "my-task", "as": "copy", "skip_deps": true, "skip_hooks": true,
	})
	data := unmarshalJSON[duplicateResult](t, result)
	if data.Branch != "custom/copy" {
		t.Errorf("branch = %q, want custom/copy", data.Branch)
	}
}

func TestDuplicateToolTrustGate(t *testing.T) {
	repoRoot := t.TempDir()
	if err := os.WriteFile(filepath.Join(repoRoot, ".gitignore"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	created := false
	r := duplicateToolRunner(repoRoot, "feature/my-task", &created, false)
	cfg := testConfig()
	cfg.PostCreate = []string{"make install"}
	hctx := &HandlerContext{Runner: r, Config: cfg, RepoRoot: repoRoot, Version: "test"}

	result := callTool(t, handleDuplicate(hctx), map[string]any{"task": "my-task"})
	if got := resultError(t, result); !strings.Contains(got, "rimba trust") {
		t.Errorf("error = %q, want trust guidance", got)
	}
	if created {
		t.Fatal("untrusted request must not create a worktree")
	}
}

func TestDuplicateToolOperationError(t *testing.T) {
	repoRoot := t.TempDir()
	created := false
	r := duplicateToolRunner(repoRoot, "feature/my-task", &created, true)
	hctx := &HandlerContext{Runner: r, Config: testConfig(), RepoRoot: repoRoot, Version: "test"}

	result := callTool(t, handleDuplicate(hctx), map[string]any{
		"task": "my-task", "as": "copy", "skip_deps": true, "skip_hooks": true,
	})
	if got := resultError(t, result); !strings.Contains(got, "already exists") {
		t.Errorf("error = %q, want operation error", got)
	}
}

func duplicateToolRunner(repoRoot, branch string, created *bool, branchExists bool) *mockRunner {
	porcelain := worktreePorcelain(
		struct{ path, branch string }{repoRoot, "main"},
		struct{ path, branch string }{filepath.Join(repoRoot, ".worktrees", strings.ReplaceAll(branch, "/", "-")), branch},
	)
	return &mockRunner{
		run: func(args ...string) (string, error) {
			switch {
			case len(args) > 1 && args[0] == gitWorktree && args[1] == gitList:
				return porcelain, nil
			case len(args) > 0 && args[0] == gitRevParse:
				if branchExists {
					return "abc", nil
				}
				return "", errors.New("not found")
			case len(args) > 1 && args[0] == gitWorktree && args[1] == gitWorktreeAdd:
				*created = true
				if err := os.MkdirAll(args[5], 0o755); err != nil {
					return "", err
				}
				return "", nil
			}
			return "", nil
		},
	}
}

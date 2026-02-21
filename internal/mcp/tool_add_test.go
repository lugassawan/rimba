package mcp

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const (
	branchFeatureMyTask = "feature/my-task"
	sourceMain          = "main"
)

func TestAddToolRequiresTask(t *testing.T) {
	hctx := testContext(&mockRunner{})
	handler := handleAdd(hctx)

	result := callTool(t, handler, nil)
	errText := resultError(t, result)
	if !strings.Contains(errText, "task is required") {
		t.Errorf("expected 'task is required', got: %s", errText)
	}
}

func TestAddToolRequiresConfig(t *testing.T) {
	hctx := &HandlerContext{
		Runner:   &mockRunner{},
		Config:   nil,
		RepoRoot: "/repo",
		Version:  "test",
	}
	handler := handleAdd(hctx)

	result := callTool(t, handler, map[string]any{"task": "test"})
	errText := resultError(t, result)
	if !strings.Contains(errText, "not initialized") {
		t.Errorf("expected config error, got: %s", errText)
	}
}

func TestAddToolInvalidType(t *testing.T) {
	hctx := testContext(&mockRunner{})
	handler := handleAdd(hctx)

	result := callTool(t, handler, map[string]any{"task": "test", "type": "invalid"})
	errText := resultError(t, result)
	if !strings.Contains(errText, "invalid type") {
		t.Errorf("expected 'invalid type' error, got: %s", errText)
	}
}

func TestAddToolBranchExists(t *testing.T) {
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			// BranchExists: "show-ref --verify" returns success
			if len(args) > 0 && args[0] == "show-ref" {
				return "abc123 refs/heads/feature/my-task", nil
			}
			return "", nil
		},
	}
	hctx := testContext(r)
	handler := handleAdd(hctx)

	result := callTool(t, handler, map[string]any{"task": "my-task"})
	errText := resultError(t, result)
	if !strings.Contains(errText, "already exists") {
		t.Errorf("expected 'already exists' error, got: %s", errText)
	}
}

func TestAddToolSuccess(t *testing.T) {
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			// BranchExists: rev-parse --verify returns error = branch doesn't exist
			if len(args) > 0 && args[0] == gitRevParse {
				return "", errors.New("not found")
			}
			// AddWorktree: worktree add
			if len(args) > 0 && args[0] == gitWorktree {
				return "", nil
			}
			return "", nil
		},
	}
	cfg := testConfig()
	cfg.CopyFiles = nil
	hctx := testContext(r)
	hctx.Config = cfg
	handler := handleAdd(hctx)

	result := callTool(t, handler, map[string]any{"task": "my-task", "skip_deps": true, "skip_hooks": true})
	data := unmarshalJSON[addResult](t, result)
	if data.Task != "my-task" {
		t.Errorf("task = %q, want %q", data.Task, "my-task")
	}
	if data.Branch != branchFeatureMyTask {
		t.Errorf("branch = %q, want %q", data.Branch, branchFeatureMyTask)
	}
	if data.Source != sourceMain {
		t.Errorf("source = %q, want %q", data.Source, sourceMain)
	}
	if !strings.Contains(data.Path, ".worktrees") {
		t.Errorf("path = %q, expected to contain worktree dir", data.Path)
	}
}

func TestAddToolSuccessCustomSource(t *testing.T) {
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			if len(args) > 0 && args[0] == gitRevParse {
				return "", errors.New("not found")
			}
			if len(args) > 0 && args[0] == gitWorktree {
				return "", nil
			}
			return "", nil
		},
	}
	cfg := testConfig()
	cfg.CopyFiles = nil
	hctx := testContext(r)
	hctx.Config = cfg
	handler := handleAdd(hctx)

	result := callTool(t, handler, map[string]any{
		"task":       "my-task",
		"source":     "develop",
		"skip_deps":  true,
		"skip_hooks": true,
	})
	data := unmarshalJSON[addResult](t, result)
	if data.Source != "develop" {
		t.Errorf("source = %q, want %q", data.Source, "develop")
	}
}

func TestAddToolSuccessSkipDeps(t *testing.T) {
	var worktreeAddCalled bool
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			if len(args) > 0 && args[0] == gitRevParse {
				return "", errors.New("not found")
			}
			if len(args) > 0 && args[0] == gitWorktree {
				worktreeAddCalled = true
				return "", nil
			}
			return "", nil
		},
	}
	cfg := testConfig()
	cfg.CopyFiles = nil
	hctx := testContext(r)
	hctx.Config = cfg
	handler := handleAdd(hctx)

	result := callTool(t, handler, map[string]any{
		"task":       "feat-1",
		"skip_deps":  true,
		"skip_hooks": true,
	})
	data := unmarshalJSON[addResult](t, result)
	if data.Task != "feat-1" {
		t.Errorf("task = %q, want %q", data.Task, "feat-1")
	}
	if !worktreeAddCalled {
		t.Error("expected worktree add to be called")
	}
}

func TestAddToolAddWorktreeError(t *testing.T) {
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			if len(args) > 0 && args[0] == gitRevParse {
				return "", errors.New("not found")
			}
			if len(args) > 0 && args[0] == gitWorktree {
				return "", errors.New("fatal: cannot create worktree")
			}
			return "", nil
		},
	}
	cfg := testConfig()
	cfg.CopyFiles = nil
	hctx := testContext(r)
	hctx.Config = cfg
	handler := handleAdd(hctx)

	result := callTool(t, handler, map[string]any{"task": "my-task", "skip_deps": true})
	errText := resultError(t, result)
	if !strings.Contains(errText, "cannot create worktree") {
		t.Errorf("expected worktree creation error, got: %s", errText)
	}
}

func TestAddToolCopyEntriesSkipsMissing(t *testing.T) {
	// CopyEntries silently skips missing source files, so this succeeds
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			if len(args) > 0 && args[0] == gitRevParse {
				return "", errors.New("not found")
			}
			if len(args) > 0 && args[0] == gitWorktree {
				return "", nil
			}
			return "", nil
		},
	}
	cfg := testConfig() // has CopyFiles: [".editorconfig"]
	hctx := testContext(r)
	hctx.Config = cfg
	handler := handleAdd(hctx)

	result := callTool(t, handler, map[string]any{"task": "my-task", "skip_deps": true, "skip_hooks": true})
	data := unmarshalJSON[addResult](t, result)
	if data.Task != "my-task" {
		t.Errorf("task = %q, want %q", data.Task, "my-task")
	}
}

func TestAddToolCopyEntriesError(t *testing.T) {
	// Create a real file in a temp dir so CopyEntries tries to actually copy it
	tmpDir := t.TempDir()
	srcFile := filepath.Join(tmpDir, "real-file.txt")
	if err := os.WriteFile(srcFile, []byte("content"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create a read-only directory to use as worktree base so MkdirAll fails
	readOnlyDir := filepath.Join(tmpDir, "readonly")
	if err := os.MkdirAll(readOnlyDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(readOnlyDir, 0555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = os.Chmod(readOnlyDir, 0755)
	})

	r := &mockRunner{
		run: func(args ...string) (string, error) {
			if len(args) > 0 && args[0] == gitRevParse {
				return "", errors.New("not found")
			}
			if len(args) > 0 && args[0] == gitWorktree {
				return "", nil
			}
			return "", nil
		},
	}
	cfg := testConfig()
	cfg.CopyFiles = []string{"real-file.txt"}
	cfg.WorktreeDir = "readonly"
	hctx := testContext(r)
	hctx.Config = cfg
	hctx.RepoRoot = tmpDir
	handler := handleAdd(hctx)

	result := callTool(t, handler, map[string]any{"task": "my-task", "skip_deps": true})
	errText := resultError(t, result)
	if !strings.Contains(errText, "failed to copy files") {
		t.Errorf("expected copy files error, got: %s", errText)
	}
}

func TestAddToolBugfixType(t *testing.T) {
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			if len(args) > 0 && args[0] == gitRevParse {
				return "", errors.New("not found")
			}
			if len(args) > 0 && args[0] == gitWorktree {
				return "", nil
			}
			return "", nil
		},
	}
	cfg := testConfig()
	cfg.CopyFiles = nil
	hctx := testContext(r)
	hctx.Config = cfg
	handler := handleAdd(hctx)

	result := callTool(t, handler, map[string]any{
		"task":       "fix-login",
		"type":       "bugfix",
		"skip_deps":  true,
		"skip_hooks": true,
	})
	data := unmarshalJSON[addResult](t, result)
	if data.Branch != "bugfix/fix-login" {
		t.Errorf("branch = %q, want %q", data.Branch, "bugfix/fix-login")
	}
}

func TestAddToolPathAlreadyExists(t *testing.T) {
	// Create a temp dir to simulate existing worktree path
	tmpDir := t.TempDir()
	wtPath := filepath.Join(tmpDir, ".worktrees", "feature-my-task")
	if err := os.MkdirAll(wtPath, 0o755); err != nil {
		t.Fatal(err)
	}

	r := &mockRunner{
		run: func(args ...string) (string, error) {
			// BranchExists: rev-parse --verify returns error = branch doesn't exist
			if len(args) > 0 && args[0] == gitRevParse {
				return "", errors.New("not found")
			}
			return "", nil
		},
	}
	cfg := testConfig()
	hctx := &HandlerContext{
		Runner:   r,
		Config:   cfg,
		RepoRoot: tmpDir,
		Version:  "test",
	}
	handler := handleAdd(hctx)

	result := callTool(t, handler, map[string]any{"task": "my-task"})
	errText := resultError(t, result)
	if !strings.Contains(errText, "already exists") {
		t.Errorf("expected 'already exists' error, got: %s", errText)
	}
}

func TestAddToolWithDepsAndHooks(t *testing.T) {
	tmpDir := t.TempDir()
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			// BranchExists: rev-parse --verify returns error = branch doesn't exist
			if len(args) > 0 && args[0] == gitRevParse {
				return "", errors.New("not found")
			}
			// AddWorktree: create the directory
			if len(args) > 0 && args[0] == gitWorktree && len(args) > 1 && args[1] == "add" {
				_ = os.MkdirAll(args[2], 0o755)
				return "", nil
			}
			// ListWorktrees for deps
			if len(args) > 0 && args[0] == gitWorktree && len(args) > 1 && args[1] == gitList {
				return "worktree " + tmpDir + "\nHEAD abc\nbranch refs/heads/main\n\n", nil
			}
			return "", nil
		},
	}
	cfg := testConfig()
	cfg.CopyFiles = nil
	cfg.PostCreate = []string{"echo hello"}
	hctx := &HandlerContext{
		Runner:   r,
		Config:   cfg,
		RepoRoot: tmpDir,
		Version:  "test",
	}
	handler := handleAdd(hctx)

	// skip_deps=false, skip_hooks=false — exercise both paths
	result := callTool(t, handler, map[string]any{"task": "with-hooks"})
	data := unmarshalJSON[addResult](t, result)
	if data.Task != "with-hooks" {
		t.Errorf("task = %q", data.Task)
	}
}

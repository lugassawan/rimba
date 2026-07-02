package mcp

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lugassawan/rimba/internal/trust"
	"github.com/lugassawan/rimba/testutil"
)

const (
	taskMyTask                 = "my-task"
	branchFeatureMyTask        = "feature/my-task"
	branchServiceFeatureMyTask = "auth-api/feature/my-task"
	sourceMain                 = "main"
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
	if data.Task != taskMyTask {
		t.Errorf("task = %q, want %q", data.Task, taskMyTask)
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
	if data.Task != taskMyTask {
		t.Errorf("task = %q, want %q", data.Task, taskMyTask)
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

func TestAddToolServiceScoped(t *testing.T) {
	tmpDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(tmpDir, "auth-api"), 0o755); err != nil {
		t.Fatal(err)
	}

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
	hctx := &HandlerContext{
		Runner:   r,
		Config:   cfg,
		RepoRoot: tmpDir,
		Version:  "test",
	}
	handler := handleAdd(hctx)

	result := callTool(t, handler, map[string]any{"task": "auth-api/my-task", "skip_deps": true, "skip_hooks": true})
	data := unmarshalJSON[addResult](t, result)
	if data.Task != taskMyTask {
		t.Errorf("task = %q, want %q", data.Task, taskMyTask)
	}
	if data.Branch != branchServiceFeatureMyTask {
		t.Errorf("branch = %q, want %q", data.Branch, branchServiceFeatureMyTask)
	}
}

func TestAddToolWithDepsAndHooks(t *testing.T) {
	t.Setenv("RIMBA_TRUST_YES", "1")
	tmpDir := t.TempDir()
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			// BranchExists: rev-parse --verify returns error = branch doesn't exist
			if len(args) > 0 && args[0] == gitRevParse {
				return "", errors.New("not found")
			}
			// AddWorktree: create the directory
			if len(args) > 0 && args[0] == gitWorktree && len(args) > 1 && args[1] == gitWorktreeAdd {
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

func TestAddToolTrustGateUntrusted(t *testing.T) {
	tmpDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmpDir, ".gitignore"), nil, 0644); err != nil {
		t.Fatal(err)
	}
	cfg := testConfig()
	cfg.PostCreate = []string{"make install"}
	hctx := &HandlerContext{
		Runner:   &mockRunner{},
		Config:   cfg,
		RepoRoot: tmpDir,
		Version:  "test",
	}
	handler := handleAdd(hctx)

	result := callTool(t, handler, map[string]any{"task": "blocked"})
	errText := resultError(t, result)
	if !strings.Contains(errText, "rimba trust") {
		t.Errorf("untrusted error should mention 'rimba trust', got: %s", errText)
	}
}

func TestAddToolTrustGatePreTrusted(t *testing.T) {
	// When trust is already recorded, the gate should pass without env hatch.
	tmpDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmpDir, ".gitignore"), nil, 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(tmpDir, ".rimba"), 0750); err != nil {
		t.Fatal(err)
	}

	cfg := testConfig()
	cfg.PostCreate = []string{"echo trusted"}

	// Pre-record trust.
	h := trust.Hash(cfg)
	if err := trust.Record(tmpDir, h); err != nil {
		t.Fatal(err)
	}

	r := &mockRunner{
		run: func(args ...string) (string, error) {
			if len(args) > 0 && args[0] == gitRevParse {
				return "", errors.New("not found")
			}
			if len(args) > 0 && args[0] == gitWorktree && len(args) > 1 && args[1] == gitWorktreeAdd {
				_ = os.MkdirAll(args[2], 0o755)
				return "", nil
			}
			if len(args) > 0 && args[0] == gitWorktree && len(args) > 1 && args[1] == gitList {
				return "worktree " + tmpDir + "\nHEAD abc\nbranch refs/heads/main\n\n", nil
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
	handler := handleAdd(hctx)
	cfg.CopyFiles = nil

	result := callTool(t, handler, map[string]any{"task": "trusted-task"})
	data := unmarshalJSON[addResult](t, result)
	if data.Task != "trusted-task" {
		t.Errorf("pre-trusted add should succeed, task = %q", data.Task)
	}
}

func TestAddToolTrustGateEnvEscapeHatch(t *testing.T) {
	t.Setenv("RIMBA_TRUST_YES", "1")
	tmpDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmpDir, ".gitignore"), nil, 0644); err != nil {
		t.Fatal(err)
	}
	cfg := testConfig()
	cfg.PostCreate = []string{"pnpm install"}
	cfg.CopyFiles = nil

	r := &mockRunner{
		run: func(args ...string) (string, error) {
			if len(args) > 0 && args[0] == gitRevParse {
				return "", errors.New("not found")
			}
			if len(args) > 0 && args[0] == gitWorktree && len(args) > 1 && args[1] == gitWorktreeAdd {
				_ = os.MkdirAll(args[2], 0o755)
				return "", nil
			}
			if len(args) > 0 && args[0] == gitWorktree && len(args) > 1 && args[1] == gitList {
				return "worktree " + tmpDir + "\nHEAD abc\nbranch refs/heads/main\n\n", nil
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
	handler := handleAdd(hctx)

	result := callTool(t, handler, map[string]any{"task": "env-bypass"})
	data := unmarshalJSON[addResult](t, result)
	if data.Task != "env-bypass" {
		t.Errorf("RIMBA_TRUST_YES add should succeed, task = %q", data.Task)
	}
}

// ── Dispatcher guard ─────────────────────────────────────────────────────────

func TestAddToolPRBranchMutuallyExclusive(t *testing.T) {
	hctx := testContext(&mockRunner{})
	handler := handleAdd(hctx)

	result := callTool(t, handler, map[string]any{"pr": 42, "branch": "feature/x"})
	errText := resultError(t, result)
	if !strings.Contains(errText, "mutually exclusive") {
		t.Errorf("expected 'mutually exclusive' error, got: %s", errText)
	}
}

// ── PR mode ──────────────────────────────────────────────────────────────────

// makePRGitRunner returns a mockRunner for the happy-path PR add.
// It simulates: fetch succeeds, BranchExists=false, worktree add succeeds.
func makePRGitRunner() *mockRunner {
	return &mockRunner{
		run: func(args ...string) (string, error) {
			switch {
			case len(args) > 0 && args[0] == "fetch":
				return "", nil
			case len(args) > 1 && args[0] == gitRevParse && args[1] == flagVerify:
				return "", errors.New("not found") // branch doesn't exist
			case len(args) > 1 && args[0] == gitWorktree && args[1] == gitWorktreeAdd:
				return "", nil
			}
			return "", nil
		},
	}
}

func TestAddPRToolSuccess(t *testing.T) {
	prJSON := testutil.LoadFixture(t, "../gh/testdata/same_repo_pr.json")
	tmpDir := t.TempDir()

	cfg := testConfig()
	cfg.CopyFiles = nil
	hctx := &HandlerContext{
		Runner:   makePRGitRunner(),
		GH:       newGhAuthOK(prJSON),
		Config:   cfg,
		RepoRoot: tmpDir,
		Version:  "test",
	}
	handler := handleAdd(hctx)

	result := callTool(t, handler, map[string]any{
		"pr":         42,
		"skip_deps":  true,
		"skip_hooks": true,
	})
	data := unmarshalJSON[addResult](t, result)
	if data.Branch != "review/42-fix-login-redirect" {
		t.Errorf("branch = %q, want %q", data.Branch, "review/42-fix-login-redirect")
	}
	if !strings.Contains(data.Path, ".worktrees") {
		t.Errorf("path = %q, expected to contain worktree dir", data.Path)
	}
}

func TestAddPRToolNegativeNumber(t *testing.T) {
	hctx := &HandlerContext{
		Runner:   &mockRunner{},
		GH:       newGhAuthOK(""),
		Config:   testConfig(),
		RepoRoot: "/repo",
		Version:  "test",
	}
	handler := handleAdd(hctx)

	result := callTool(t, handler, map[string]any{"pr": -1})
	errText := resultError(t, result)
	if !strings.Contains(errText, "invalid pr number") {
		t.Errorf("expected 'invalid pr number' error, got: %s", errText)
	}
}

func TestAddPRToolNilGHRunner(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := testConfig()
	cfg.CopyFiles = nil
	hctx := &HandlerContext{
		Runner:   &mockRunner{},
		GH:       nil, // intentionally unset
		Config:   cfg,
		RepoRoot: tmpDir,
		Version:  "test",
	}
	handler := handleAdd(hctx)

	result := callTool(t, handler, map[string]any{"pr": 42})
	errText := resultError(t, result)
	if !strings.Contains(errText, "server startup bug") {
		t.Errorf("expected startup bug error for nil GH, got: %s", errText)
	}
}

func TestAddPRToolUntrustedRepo(t *testing.T) {
	tmpDir := t.TempDir()
	prJSON := testutil.LoadFixture(t, "../gh/testdata/same_repo_pr.json")
	cfg := testConfig()
	cfg.CopyFiles = nil
	cfg.PostCreate = []string{"make install"}
	hctx := &HandlerContext{
		Runner:   &mockRunner{},
		GH:       newGhAuthOK(prJSON),
		Config:   cfg,
		RepoRoot: tmpDir,
		Version:  "test",
	}
	handler := handleAdd(hctx)

	result := callTool(t, handler, map[string]any{"pr": 42})
	errText := resultError(t, result)
	if !strings.Contains(errText, "rimba trust") {
		t.Errorf("untrusted PR add error should mention 'rimba trust', got: %s", errText)
	}
}

func TestAddPRToolFetchError(t *testing.T) {
	ghR := &mockGhRunner{
		run: func(_ context.Context, args ...string) ([]byte, error) {
			if len(args) > 0 && args[0] == "auth" {
				return []byte("Logged in"), nil
			}
			return nil, errors.New("HTTP 404: Not Found")
		},
	}
	tmpDir := t.TempDir()
	cfg := testConfig()
	cfg.CopyFiles = nil
	hctx := &HandlerContext{
		Runner:   &mockRunner{},
		GH:       ghR,
		Config:   cfg,
		RepoRoot: tmpDir,
		Version:  "test",
	}
	handler := handleAdd(hctx)

	result := callTool(t, handler, map[string]any{"pr": 999, "skip_deps": true, "skip_hooks": true})
	errText := resultError(t, result)
	if !strings.Contains(errText, "verify PR number") {
		t.Errorf("expected 'verify PR number' hint, got: %s", errText)
	}
}

func TestAddPRToolTaskOverride(t *testing.T) {
	prJSON := testutil.LoadFixture(t, "../gh/testdata/same_repo_pr.json")
	tmpDir := t.TempDir()

	cfg := testConfig()
	cfg.CopyFiles = nil
	hctx := &HandlerContext{
		Runner:   makePRGitRunner(),
		GH:       newGhAuthOK(prJSON),
		Config:   cfg,
		RepoRoot: tmpDir,
		Version:  "test",
	}
	handler := handleAdd(hctx)

	result := callTool(t, handler, map[string]any{
		"pr":         42,
		"task":       "my-review",
		"skip_deps":  true,
		"skip_hooks": true,
	})
	data := unmarshalJSON[addResult](t, result)
	if data.Branch != "my-review" {
		t.Errorf("branch with task override = %q, want %q", data.Branch, "my-review")
	}
}

// ── Branch promotion mode ─────────────────────────────────────────────────────

// makePromoteRunner returns a mockRunner for the happy-path branch promotion.
// It simulates: default branch resolves to main, branch exists, not in other
// worktrees, HEAD is the target branch, working tree clean, checkout and
// worktree add both succeed.
// Matching on first arg only keeps cyclomatic complexity well under the 15 limit.
func makePromoteRunner(repoRoot, branch string) *mockRunner {
	porcelain := "worktree " + repoRoot + "\nHEAD abc\nbranch refs/heads/main\n\n"
	return &mockRunner{
		run: func(args ...string) (string, error) {
			if len(args) == 0 {
				return "", nil
			}
			switch args[0] {
			case gitSymbolicRef:
				return refsOriginMain, nil
			case gitRevParse:
				return "", nil // BranchExists → true (nil error)
			case gitWorktree:
				return porcelain, nil // covers list --porcelain and add
			}
			return "", nil
		},
		runInDir: func(_ string, args ...string) (string, error) {
			if len(args) == 0 {
				return "", nil
			}
			switch args[0] {
			case gitSymbolicRef:
				return branch, nil // CurrentBranch via symbolic-ref --short HEAD
			case gitStatus:
				return "", nil // IsDirty → clean
			}
			return "", nil // Checkout (switch -- main) and anything else
		},
	}
}

func TestAddBranchToolSuccess(t *testing.T) {
	tmpDir := t.TempDir()
	branch := "feature/promote-me"

	cfg := testConfig()
	cfg.CopyFiles = nil
	hctx := &HandlerContext{
		Runner:   makePromoteRunner(tmpDir, branch),
		Config:   cfg,
		RepoRoot: tmpDir,
		Version:  "test",
	}
	handler := handleAdd(hctx)

	result := callTool(t, handler, map[string]any{"branch": branch})
	data := unmarshalJSON[addResult](t, result)
	if data.Branch != branch {
		t.Errorf("branch = %q, want %q", data.Branch, branch)
	}
	if !strings.Contains(data.Path, ".worktrees") {
		t.Errorf("path = %q, expected to contain worktree dir", data.Path)
	}
}

func TestAddBranchToolValidationError(t *testing.T) {
	tmpDir := t.TempDir()

	// BranchExists returns false → validateForPromotion errors with "does not exist"
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			if len(args) >= 1 && args[0] == gitSymbolicRef {
				return refsOriginMain, nil // DefaultBranch → "main"
			}
			if len(args) >= 2 && args[0] == gitRevParse && args[1] == flagVerify {
				return "", errors.New("not found") // BranchExists → false
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

	result := callTool(t, handler, map[string]any{"branch": "feature/no-such"})
	errText := resultError(t, result)
	if !strings.Contains(errText, "does not exist") {
		t.Errorf("expected 'does not exist' error, got: %s", errText)
	}
}

func TestAddBranchToolPathAlreadyExists(t *testing.T) {
	tmpDir := t.TempDir()
	branch := "feature/promote-me"

	// Pre-create the expected worktree path so PromoteBranch returns "already exists".
	wtPath := filepath.Join(tmpDir, ".worktrees", "feature-promote-me")
	if err := os.MkdirAll(wtPath, 0o755); err != nil {
		t.Fatal(err)
	}

	cfg := testConfig()
	cfg.CopyFiles = nil
	hctx := &HandlerContext{
		Runner:   makePromoteRunner(tmpDir, branch),
		Config:   cfg,
		RepoRoot: tmpDir,
		Version:  "test",
	}
	handler := handleAdd(hctx)

	result := callTool(t, handler, map[string]any{"branch": branch})
	errText := resultError(t, result)
	if !strings.Contains(errText, "already exists") {
		t.Errorf("expected 'already exists' error, got: %s", errText)
	}
}

func TestAddBranchToolRequiresConfig(t *testing.T) {
	hctx := &HandlerContext{
		Runner:   &mockRunner{},
		Config:   nil,
		RepoRoot: "/repo",
		Version:  "test",
	}
	handler := handleAdd(hctx)

	result := callTool(t, handler, map[string]any{"branch": "feature/x"})
	errText := resultError(t, result)
	if !strings.Contains(errText, "not initialized") {
		t.Errorf("expected config error, got: %s", errText)
	}
}

func TestAddPRToolRequiresConfig(t *testing.T) {
	hctx := &HandlerContext{
		Runner:   &mockRunner{},
		Config:   nil,
		RepoRoot: "/repo",
		Version:  "test",
	}
	handler := handleAdd(hctx)

	result := callTool(t, handler, map[string]any{"pr": 42})
	errText := resultError(t, result)
	if !strings.Contains(errText, "not initialized") {
		t.Errorf("expected config error, got: %s", errText)
	}
}

package mcp

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/lugassawan/rimba/internal/config"
)

const (
	mergeCmd       = "merge"
	dirtyFileEntry = " M dirty-file.go"
)

func TestMergeToolRequiresSource(t *testing.T) {
	hctx := testContext(&mockRunner{})
	handler := handleMerge(hctx)

	result := callTool(t, handler, nil)
	errText := resultError(t, result)
	if !strings.Contains(errText, "source is required") {
		t.Errorf("expected 'source is required', got: %s", errText)
	}
}

func TestMergeToolRequiresConfig(t *testing.T) {
	hctx := &HandlerContext{
		Runner:   &mockRunner{},
		Config:   nil,
		RepoRoot: "/repo",
		Version:  "test",
	}
	handler := handleMerge(hctx)

	result := callTool(t, handler, map[string]any{"source": "my-task"})
	errText := resultError(t, result)
	if !strings.Contains(errText, "not initialized") {
		t.Errorf("expected config error, got: %s", errText)
	}
}

func TestMergeToolRejectsInvalidConfig(t *testing.T) {
	hctx := &HandlerContext{
		Runner:   &mockRunner{},
		Config:   &config.Config{CommandTimeout: "notaduration"},
		RepoRoot: "/repo",
		Version:  "test",
	}
	handler := handleMerge(hctx)

	result := callTool(t, handler, map[string]any{"source": "my-task"})
	errText := resultError(t, result)
	if !strings.Contains(errText, "command_timeout") {
		t.Errorf("expected command_timeout validation error, got: %s", errText)
	}
}

func TestMergeToolSourceNotFound(t *testing.T) {
	porcelain := worktreePorcelain(
		struct{ path, branch string }{"/repo", "main"},
	)

	r := &mockRunner{
		run: func(args ...string) (string, error) {
			return porcelain, nil
		},
	}
	hctx := testContext(r)
	handler := handleMerge(hctx)

	result := callTool(t, handler, map[string]any{"source": "nonexistent"})
	errText := resultError(t, result)
	if !strings.Contains(errText, "not found") {
		t.Errorf("expected 'not found' error, got: %s", errText)
	}
}

func TestMergeToolOrphanedSourceHardErrors(t *testing.T) {
	// TASK- is the only configured prefix, so orphaned PROJ-* triggers a hard
	// error here since this tool has no force param to bypass the guard.
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
	handler := handleMerge(hctx)

	result := callTool(t, handler, map[string]any{"source": "PROJ-123"})
	errText := resultError(t, result)
	if !strings.Contains(errText, "re-add the prefix") {
		t.Errorf("expected orphan-guard error mentioning re-adding the prefix, got: %s", errText)
	}
}

// mergeHappyPathRunner builds a mock runner for the happy-path merge-to-main
// scenario. The boolean pointers let callers observe which git operations ran.
func mergeHappyPathRunner(porcelain string, removedCalled *bool) *mockRunner {
	return &mockRunner{
		run: mergeHappyPathRun(porcelain, removedCalled),
		runInDir: func(dir string, args ...string) (string, error) {
			if len(args) >= 1 && args[0] == gitStatus {
				return "", nil
			}
			if len(args) >= 1 && args[0] == mergeCmd {
				return "", nil
			}
			return "", nil
		},
	}
}

func mergeHappyPathRun(porcelain string, removedCalled *bool) func(args ...string) (string, error) {
	return func(args ...string) (string, error) {
		if len(args) >= 2 && args[0] == gitWorktree && args[1] == gitList {
			return porcelain, nil
		}
		if len(args) >= 2 && args[0] == gitWorktree && args[1] == gitRemove {
			if removedCalled != nil {
				*removedCalled = true
			}
			return "", nil
		}
		if len(args) >= 2 && args[0] == gitBranch && args[1] == "-D" {
			return "", nil
		}
		return "", nil
	}
}

func TestMergeToolHappyPathMergeToMain(t *testing.T) {
	porcelain := worktreePorcelain(
		struct{ path, branch string }{"/repo", "main"},
		struct{ path, branch string }{"/repo/.worktrees/feature-my-task", branchFeatureMyTask},
	)

	r := mergeHappyPathRunner(porcelain, nil)
	hctx := testContext(r)
	handler := handleMerge(hctx)

	result := callTool(t, handler, map[string]any{"source": "my-task"})
	data := unmarshalJSON[mergeResult](t, result)

	if data.Source != branchFeatureMyTask {
		t.Errorf("expected source 'feature/my-task', got %q", data.Source)
	}
	if data.Into != sourceMain {
		t.Errorf("expected into 'main', got %q", data.Into)
	}
	if !data.SourceRemoved {
		t.Errorf("expected source to be removed (auto-cleanup)")
	}
}

func TestMergeToolKeepSourceWorktree(t *testing.T) {
	porcelain := worktreePorcelain(
		struct{ path, branch string }{"/repo", "main"},
		struct{ path, branch string }{"/repo/.worktrees/feature-my-task", branchFeatureMyTask},
	)

	var removeWorktreeCalled bool
	r := mergeHappyPathRunner(porcelain, &removeWorktreeCalled)
	hctx := testContext(r)
	handler := handleMerge(hctx)

	result := callTool(t, handler, map[string]any{"source": "my-task", "keep": true})
	data := unmarshalJSON[mergeResult](t, result)

	if data.SourceRemoved {
		t.Errorf("expected source NOT to be removed when keep=true")
	}
	if removeWorktreeCalled {
		t.Errorf("worktree remove should not have been called when keep=true")
	}
}

func TestMergeToolIntoAnotherWorktree(t *testing.T) {
	porcelain := worktreePorcelain(
		struct{ path, branch string }{"/repo", "main"},
		struct{ path, branch string }{"/repo/.worktrees/feature-source", "feature/source"},
		struct{ path, branch string }{"/repo/.worktrees/feature-target", "feature/target"},
	)

	var mergeDir string
	var mergeBranch string
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			if len(args) >= 2 && args[0] == gitWorktree && args[1] == gitList {
				return porcelain, nil
			}
			return "", nil
		},
		runInDir: func(dir string, args ...string) (string, error) {
			if len(args) >= 1 && args[0] == gitStatus {
				return "", nil
			}
			if len(args) >= 1 && args[0] == mergeCmd {
				mergeDir = dir
				mergeBranch = args[len(args)-1]
				return "", nil
			}
			return "", nil
		},
	}
	hctx := testContext(r)
	handler := handleMerge(hctx)

	result := callTool(t, handler, map[string]any{"source": "source", "into": "target"})
	data := unmarshalJSON[mergeResult](t, result)

	if data.Source != "feature/source" {
		t.Errorf("expected source 'feature/source', got %q", data.Source)
	}
	if data.Into != "feature/target" {
		t.Errorf("expected into 'feature/target', got %q", data.Into)
	}
	if data.SourceRemoved {
		t.Errorf("expected source NOT to be removed when merging into another worktree (delete not set)")
	}
	if mergeDir != "/repo/.worktrees/feature-target" {
		t.Errorf("expected merge to run in target dir, got %q", mergeDir)
	}
	if mergeBranch != "feature/source" {
		t.Errorf("expected merge branch 'feature/source', got %q", mergeBranch)
	}
}

func TestMergeToolIntoAnotherWorktreeWithDelete(t *testing.T) {
	porcelain := worktreePorcelain(
		struct{ path, branch string }{"/repo", "main"},
		struct{ path, branch string }{"/repo/.worktrees/feature-source", "feature/source"},
		struct{ path, branch string }{"/repo/.worktrees/feature-target", "feature/target"},
	)

	r := mergeHappyPathRunner(porcelain, nil)
	hctx := testContext(r)
	handler := handleMerge(hctx)

	result := callTool(t, handler, map[string]any{"source": "source", "into": "target", "delete": true})
	data := unmarshalJSON[mergeResult](t, result)

	if !data.SourceRemoved {
		t.Errorf("expected source to be removed when delete=true")
	}
}

func TestMergeToolTargetNotFound(t *testing.T) {
	porcelain := worktreePorcelain(
		struct{ path, branch string }{"/repo", "main"},
		struct{ path, branch string }{"/repo/.worktrees/feature-source", "feature/source"},
	)

	r := &mockRunner{
		run: func(args ...string) (string, error) {
			return porcelain, nil
		},
	}
	hctx := testContext(r)
	handler := handleMerge(hctx)

	result := callTool(t, handler, map[string]any{"source": "source", "into": "nonexistent"})
	errText := resultError(t, result)
	if !strings.Contains(errText, "not found") {
		t.Errorf("expected 'not found' error, got: %s", errText)
	}
}

func TestMergeToolDirtyChecks(t *testing.T) {
	tests := []struct {
		name     string
		dirtyDir string
		contains string
	}{
		{
			name:     "SourceDirty",
			dirtyDir: "/repo/.worktrees/feature-my-task",
			contains: "my-task",
		},
		{
			name:     "TargetDirty",
			dirtyDir: "/repo",
			contains: "main",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			porcelain := worktreePorcelain(
				struct{ path, branch string }{"/repo", "main"},
				struct{ path, branch string }{"/repo/.worktrees/feature-my-task", branchFeatureMyTask},
			)

			r := &mockRunner{
				run: func(args ...string) (string, error) {
					return porcelain, nil
				},
				runInDir: func(dir string, args ...string) (string, error) {
					if len(args) >= 1 && args[0] == gitStatus {
						if dir == tc.dirtyDir {
							return dirtyFileEntry, nil
						}
						return "", nil
					}
					return "", nil
				},
			}
			hctx := testContext(r)
			handler := handleMerge(hctx)

			result := callTool(t, handler, map[string]any{"source": "my-task"})
			errText := resultError(t, result)
			if !strings.Contains(errText, "uncommitted changes") {
				t.Errorf("expected dirty error, got: %s", errText)
			}
			if !strings.Contains(errText, tc.contains) {
				t.Errorf("expected error to mention %q, got: %s", tc.contains, errText)
			}
		})
	}
}

func TestMergeToolTargetDirtyIntoWorktree(t *testing.T) {
	porcelain := worktreePorcelain(
		struct{ path, branch string }{"/repo", "main"},
		struct{ path, branch string }{"/repo/.worktrees/feature-source", "feature/source"},
		struct{ path, branch string }{"/repo/.worktrees/feature-target", "feature/target"},
	)

	r := &mockRunner{
		run: func(args ...string) (string, error) {
			return porcelain, nil
		},
		runInDir: func(dir string, args ...string) (string, error) {
			if len(args) >= 1 && args[0] == gitStatus {
				if dir == "/repo/.worktrees/feature-target" {
					return dirtyFileEntry, nil
				}
				return "", nil
			}
			return "", nil
		},
	}
	hctx := testContext(r)
	handler := handleMerge(hctx)

	result := callTool(t, handler, map[string]any{"source": "source", "into": "target"})
	errText := resultError(t, result)
	if !strings.Contains(errText, "uncommitted changes") {
		t.Errorf("expected dirty error, got: %s", errText)
	}
	if !strings.Contains(errText, "target") {
		t.Errorf("expected error to mention intoTask, got: %s", errText)
	}
}

func TestMergeToolMergeFails(t *testing.T) {
	porcelain := worktreePorcelain(
		struct{ path, branch string }{"/repo", "main"},
		struct{ path, branch string }{"/repo/.worktrees/feature-my-task", branchFeatureMyTask},
	)

	r := &mockRunner{
		run: func(args ...string) (string, error) {
			return porcelain, nil
		},
		runInDir: func(dir string, args ...string) (string, error) {
			if len(args) >= 1 && args[0] == gitStatus {
				return "", nil
			}
			if len(args) >= 1 && args[0] == mergeCmd {
				return "", errors.New("merge conflict")
			}
			return "", nil
		},
	}
	hctx := testContext(r)
	handler := handleMerge(hctx)

	result := callTool(t, handler, map[string]any{"source": "my-task"})
	errText := resultError(t, result)
	if !strings.Contains(errText, "merge conflict") {
		t.Errorf("expected merge conflict error, got: %s", errText)
	}
}

func TestMergeToolRemoveWorktreeFails(t *testing.T) {
	// A real .git file makes this a genuine (non-orphaned) failure, so it
	// short-circuits instead of routing through the heal-and-retry path.
	sourceDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(sourceDir, ".git"), []byte("gitdir: /somewhere/.git/worktrees/my-task\n"), 0o644); err != nil {
		t.Fatalf("failed to create .git fixture: %v", err)
	}
	porcelain := worktreePorcelain(
		struct{ path, branch string }{"/repo", "main"},
		struct{ path, branch string }{sourceDir, branchFeatureMyTask},
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
		runInDir: func(dir string, args ...string) (string, error) {
			if len(args) >= 1 && args[0] == gitStatus {
				return "", nil
			}
			if len(args) >= 1 && args[0] == mergeCmd {
				return "", nil
			}
			return "", nil
		},
	}
	hctx := testContext(r)
	handler := handleMerge(hctx)

	result := callTool(t, handler, map[string]any{"source": "my-task"})
	data := unmarshalJSON[mergeResult](t, result)

	if data.Source != branchFeatureMyTask {
		t.Errorf("expected source 'feature/my-task', got %q", data.Source)
	}
	if data.SourceRemoved {
		t.Errorf("expected source NOT removed when worktree remove fails")
	}
}

func TestMergeToolDeleteBranchFails(t *testing.T) {
	porcelain := worktreePorcelain(
		struct{ path, branch string }{"/repo", "main"},
		struct{ path, branch string }{"/repo/.worktrees/feature-my-task", branchFeatureMyTask},
	)

	r := &mockRunner{
		run: func(args ...string) (string, error) {
			if len(args) >= 2 && args[0] == gitWorktree && args[1] == gitList {
				return porcelain, nil
			}
			if len(args) >= 2 && args[0] == gitWorktree && args[1] == gitRemove {
				return "", nil
			}
			if len(args) >= 2 && args[0] == gitBranch && args[1] == "-D" {
				return "", errors.New("branch delete failed")
			}
			return "", nil
		},
		runInDir: func(dir string, args ...string) (string, error) {
			if len(args) >= 1 && args[0] == gitStatus {
				return "", nil
			}
			if len(args) >= 1 && args[0] == mergeCmd {
				return "", nil
			}
			return "", nil
		},
	}
	hctx := testContext(r)
	handler := handleMerge(hctx)

	result := callTool(t, handler, map[string]any{"source": "my-task"})
	data := unmarshalJSON[mergeResult](t, result)

	if data.SourceRemoved {
		t.Errorf("expected source NOT removed when branch delete fails")
	}
}

func TestMergeToolListWorktreesFails(t *testing.T) {
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			if len(args) >= 2 && args[0] == gitWorktree && args[1] == gitList {
				return "", errors.New("worktree list failed")
			}
			return "", nil
		},
	}
	hctx := testContext(r)
	handler := handleMerge(hctx)

	result := callTool(t, handler, map[string]any{"source": "my-task"})
	errText := resultError(t, result)
	if !strings.Contains(errText, "worktree list failed") {
		t.Errorf("expected worktree list error, got: %s", errText)
	}
}

// mergeNoFFRunner builds a mock runner for the no-ff test, capturing the merge
// arguments via the provided pointer.
func mergeNoFFRunner(porcelain string, mergeArgs *[]string) *mockRunner {
	return &mockRunner{
		run: mergeHappyPathRun(porcelain, nil),
		runInDir: func(dir string, args ...string) (string, error) {
			if len(args) >= 1 && args[0] == gitStatus {
				return "", nil
			}
			if len(args) >= 1 && args[0] == mergeCmd {
				*mergeArgs = args
				return "", nil
			}
			return "", nil
		},
	}
}

func TestMergeToolServiceScoped(t *testing.T) {
	tmpDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(tmpDir, "auth-api"), 0o755); err != nil {
		t.Fatal(err)
	}

	porcelain := worktreePorcelain(
		struct{ path, branch string }{tmpDir, "main"},
		struct{ path, branch string }{tmpDir + "/.worktrees/auth-api-feature-my-task", branchServiceFeatureMyTask},
	)

	r := mergeHappyPathRunner(porcelain, nil)
	hctx := &HandlerContext{
		Runner:   r,
		Config:   testConfig(),
		RepoRoot: tmpDir,
		Version:  "test",
	}
	handler := handleMerge(hctx)

	result := callTool(t, handler, map[string]any{"source": "auth-api/my-task"})
	data := unmarshalJSON[mergeResult](t, result)

	if data.Source != branchServiceFeatureMyTask {
		t.Errorf("expected source 'auth-api/feature/my-task', got %q", data.Source)
	}
	if data.Into != sourceMain {
		t.Errorf("expected into 'main', got %q", data.Into)
	}
	if !data.SourceRemoved {
		t.Errorf("expected source to be removed (auto-cleanup)")
	}
}

func TestMergeToolNoFF(t *testing.T) {
	porcelain := worktreePorcelain(
		struct{ path, branch string }{"/repo", "main"},
		struct{ path, branch string }{"/repo/.worktrees/feature-my-task", branchFeatureMyTask},
	)

	var mergeArgs []string
	r := mergeNoFFRunner(porcelain, &mergeArgs)
	hctx := testContext(r)
	handler := handleMerge(hctx)

	callTool(t, handler, map[string]any{"source": "my-task", "no_ff": true})

	if !slices.Contains(mergeArgs, "--no-ff") {
		t.Errorf("expected --no-ff in merge args, got: %v", mergeArgs)
	}
}

package mcp

import (
	"errors"
	"slices"
	"strings"
	"testing"
)

const (
	mergeCmd       = "merge"
	dirtyFileEntry = " M dirty-file.go"
	dirtyFile      = " M file.go"
	targetDir      = "/target"
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

// mergeHappyPathRunner builds a mock runner for the happy-path merge-to-main
// scenario. The boolean pointers let callers observe which git operations ran.
func mergeHappyPathRunner(porcelain string, mergedCalled, removedCalled, deletedCalled *bool) *mockRunner {
	return &mockRunner{
		run: mergeHappyPathRun(porcelain, removedCalled, deletedCalled),
		runInDir: func(dir string, args ...string) (string, error) {
			if len(args) >= 1 && args[0] == gitStatus {
				return "", nil
			}
			if len(args) >= 1 && args[0] == mergeCmd {
				if mergedCalled != nil {
					*mergedCalled = true
				}
				return "", nil
			}
			return "", nil
		},
	}
}

func mergeHappyPathRun(porcelain string, removedCalled, deletedCalled *bool) func(args ...string) (string, error) {
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
			if deletedCalled != nil {
				*deletedCalled = true
			}
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

	r := mergeHappyPathRunner(porcelain, nil, nil, nil)
	hctx := testContext(r)
	handler := handleMerge(hctx)

	result := callTool(t, handler, map[string]any{"source": "my-task"})
	data := unmarshalJSON[mergeResult](t, result)

	if data.Source != branchFeatureMyTask {
		t.Errorf("expected source 'feature/my-task', got %q", data.Source)
	}
	if data.Into != "main" {
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
	r := mergeHappyPathRunner(porcelain, nil, &removeWorktreeCalled, nil)
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

	r := mergeHappyPathRunner(porcelain, nil, nil, nil)
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

func TestCheckMergeDirtyBothClean(t *testing.T) {
	r := &mockRunner{
		runInDir: func(dir string, args ...string) (string, error) {
			return "", nil
		},
	}
	msg := checkMergeDirty(r, "/repo/.worktrees/feature-src", "/repo", "src", "", true, "main")
	if msg != "" {
		t.Errorf("expected empty string for clean worktrees, got: %s", msg)
	}
}

func TestCheckMergeDirtySourceDirty(t *testing.T) {
	r := &mockRunner{
		runInDir: func(dir string, args ...string) (string, error) {
			if dir == "/source" {
				return dirtyFile, nil
			}
			return "", nil
		},
	}
	msg := checkMergeDirty(r, "/source", targetDir, "src-task", "", true, "main")
	if !strings.Contains(msg, "src-task") {
		t.Errorf("expected source task in message, got: %s", msg)
	}
	if !strings.Contains(msg, "uncommitted") {
		t.Errorf("expected 'uncommitted' in message, got: %s", msg)
	}
}

func TestCheckMergeDirtyTargetDirty(t *testing.T) {
	r := &mockRunner{
		runInDir: func(dir string, args ...string) (string, error) {
			if dir == targetDir {
				return dirtyFile, nil
			}
			return "", nil
		},
	}
	msg := checkMergeDirty(r, "/source", targetDir, "src-task", "", true, "main")
	if !strings.Contains(msg, "main") {
		t.Errorf("expected target label in message, got: %s", msg)
	}
}

func TestCheckMergeDirtyTargetDirtyNotMain(t *testing.T) {
	r := &mockRunner{
		runInDir: func(dir string, args ...string) (string, error) {
			if dir == targetDir {
				return dirtyFile, nil
			}
			return "", nil
		},
	}
	msg := checkMergeDirty(r, "/source", targetDir, "src-task", "tgt-task", false, "feature/target")
	if !strings.Contains(msg, "tgt-task") {
		t.Errorf("expected intoTask in message when not merging to main, got: %s", msg)
	}
}

func TestCheckMergeDirtySourceError(t *testing.T) {
	r := &mockRunner{
		runInDir: func(dir string, args ...string) (string, error) {
			if dir == "/source" {
				return "", errors.New("source status error")
			}
			return "", nil
		},
	}
	msg := checkMergeDirty(r, "/source", targetDir, "src-task", "", true, "main")
	if !strings.Contains(msg, "source status error") {
		t.Errorf("expected source error in message, got: %s", msg)
	}
}

func TestCheckMergeDirtyTargetError(t *testing.T) {
	r := &mockRunner{
		runInDir: func(dir string, args ...string) (string, error) {
			if dir == targetDir {
				return "", errors.New("target status error")
			}
			return "", nil
		},
	}
	msg := checkMergeDirty(r, "/source", targetDir, "src-task", "", true, "main")
	if !strings.Contains(msg, "target status error") {
		t.Errorf("expected target error in message, got: %s", msg)
	}
}

// mergeNoFFRunner builds a mock runner for the no-ff test, capturing the merge
// arguments via the provided pointer.
func mergeNoFFRunner(porcelain string, mergeArgs *[]string) *mockRunner {
	return &mockRunner{
		run: mergeHappyPathRun(porcelain, nil, nil),
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

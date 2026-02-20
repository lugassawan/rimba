package mcp

import (
	"errors"
	"strings"
	"testing"

	"github.com/lugassawan/rimba/internal/operations"
)

const (
	rebaseCmd         = "rebase"
	pushCmd           = "push"
	originFeatureTask = "origin/feature/my-task"
)

func TestSyncToolRequiresTaskOrAll(t *testing.T) {
	hctx := testContext(&mockRunner{})
	handler := handleSync(hctx)

	result := callTool(t, handler, nil)
	errText := resultError(t, result)
	if !strings.Contains(errText, "provide a task name or set all=true") {
		t.Errorf("expected selector error, got: %s", errText)
	}
}

func TestSyncToolRequiresConfig(t *testing.T) {
	hctx := &HandlerContext{
		Runner:   &mockRunner{},
		Config:   nil,
		RepoRoot: "/repo",
		Version:  "test",
	}
	handler := handleSync(hctx)

	result := callTool(t, handler, map[string]any{"all": true})
	errText := resultError(t, result)
	if !strings.Contains(errText, "not initialized") {
		t.Errorf("expected config error, got: %s", errText)
	}
}

func TestSyncToolTaskNotFound(t *testing.T) {
	porcelain := worktreePorcelain(
		struct{ path, branch string }{"/repo", "main"},
	)

	r := &mockRunner{
		run: func(args ...string) (string, error) {
			// Fetch fails (no remote) -- that's fine
			if len(args) > 0 && args[0] == gitFetch {
				return "", nil
			}
			return porcelain, nil
		},
	}
	hctx := testContext(r)
	handler := handleSync(hctx)

	result := callTool(t, handler, map[string]any{"task": "nonexistent"})
	errText := resultError(t, result)
	if !strings.Contains(errText, "not found") {
		t.Errorf("expected 'not found' error, got: %s", errText)
	}
}

// syncMockConfig configures the behaviour of a sync mock runner.
type syncMockConfig struct {
	rebaseFails bool
	pushFails   bool
	dirty       bool
	statusError bool
	useMerge    bool
	noUpstream  bool
}

// newSyncMockRunner creates a mock runner for sync tests. The porcelain string
// provides worktree list output, and cfg controls which git operations succeed
// or fail. The optional pushCalled/mergeCalled pointers let callers observe
// whether specific operations were invoked.
func newSyncMockRunner(porcelain string, cfg syncMockConfig, pushCalled, mergeCalled *bool) *mockRunner {
	return &mockRunner{
		run: func(args ...string) (string, error) {
			if len(args) > 0 && args[0] == gitFetch {
				return "", nil
			}
			if len(args) >= 2 && args[0] == gitWorktree && args[1] == gitList {
				return porcelain, nil
			}
			return "", nil
		},
		runInDir: newSyncRunInDir(cfg, pushCalled, mergeCalled),
	}
}

func syncMockStatus(cfg syncMockConfig) (string, error) {
	if cfg.statusError {
		return "", errors.New("status check failed")
	}
	if cfg.dirty {
		return " M dirty.go", nil
	}
	return "", nil
}

func syncMockPush(cfg syncMockConfig, pushCalled *bool) (string, error) {
	if pushCalled != nil {
		*pushCalled = true
	}
	if cfg.pushFails {
		return "", errors.New("push rejected")
	}
	return "", nil
}

func newSyncRunInDir(cfg syncMockConfig, pushCalled, mergeCalled *bool) func(dir string, args ...string) (string, error) {
	return func(dir string, args ...string) (string, error) {
		if len(args) < 1 {
			return "", nil
		}
		switch args[0] {
		case gitStatus:
			return syncMockStatus(cfg)
		case rebaseCmd:
			if cfg.rebaseFails {
				if len(args) >= 2 && args[1] == "--abort" {
					return "", nil
				}
				return "", errors.New("rebase conflict")
			}
			return "", nil
		case mergeCmd:
			if mergeCalled != nil {
				*mergeCalled = true
			}
			return "", nil
		case gitRevParse:
			if cfg.noUpstream {
				return "", errors.New("no upstream")
			}
			return originFeatureTask, nil
		case pushCmd:
			return syncMockPush(cfg, pushCalled)
		default:
			return "", nil
		}
	}
}

func TestSyncSingleSuccess(t *testing.T) {
	porcelain := worktreePorcelain(
		struct{ path, branch string }{"/repo", "main"},
		struct{ path, branch string }{"/repo/.worktrees/feature-my-task", "feature/my-task"},
	)

	r := newSyncMockRunner(porcelain, syncMockConfig{}, nil, nil)
	hctx := testContext(r)
	handler := handleSync(hctx)

	result := callTool(t, handler, map[string]any{"task": "my-task"})
	data := unmarshalJSON[syncResult](t, result)

	if len(data.Results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(data.Results))
	}
	sr := data.Results[0]
	if sr.Branch != "feature/my-task" {
		t.Errorf("expected branch 'feature/my-task', got %q", sr.Branch)
	}
	if !sr.Synced {
		t.Errorf("expected synced=true")
	}
	if sr.Skipped {
		t.Errorf("expected skipped=false")
	}
	if sr.Failed {
		t.Errorf("expected failed=false")
	}
	if !sr.Pushed {
		t.Errorf("expected pushed=true")
	}
}

func TestSyncSingleNoPush(t *testing.T) {
	porcelain := worktreePorcelain(
		struct{ path, branch string }{"/repo", "main"},
		struct{ path, branch string }{"/repo/.worktrees/feature-my-task", "feature/my-task"},
	)

	var pushCalled bool
	r := newSyncMockRunner(porcelain, syncMockConfig{}, &pushCalled, nil)
	hctx := testContext(r)
	handler := handleSync(hctx)

	result := callTool(t, handler, map[string]any{"task": "my-task", "no_push": true})
	data := unmarshalJSON[syncResult](t, result)

	if len(data.Results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(data.Results))
	}
	sr := data.Results[0]
	if !sr.Synced {
		t.Errorf("expected synced=true")
	}
	if sr.Pushed {
		t.Errorf("expected pushed=false with no_push")
	}
	if pushCalled {
		t.Errorf("push should not have been called with no_push=true")
	}
}

func TestSyncSingleMergeMode(t *testing.T) {
	porcelain := worktreePorcelain(
		struct{ path, branch string }{"/repo", "main"},
		struct{ path, branch string }{"/repo/.worktrees/feature-my-task", "feature/my-task"},
	)

	var usedMerge bool
	r := newSyncMockRunner(porcelain, syncMockConfig{useMerge: true}, nil, &usedMerge)
	hctx := testContext(r)
	handler := handleSync(hctx)

	result := callTool(t, handler, map[string]any{"task": "my-task", "merge": true})
	data := unmarshalJSON[syncResult](t, result)

	if !usedMerge {
		t.Errorf("expected merge to be used instead of rebase")
	}
	if len(data.Results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(data.Results))
	}
	if !data.Results[0].Synced {
		t.Errorf("expected synced=true")
	}
}

func TestSyncMultipleAllWorktrees(t *testing.T) {
	porcelain := worktreePorcelain(
		struct{ path, branch string }{"/repo", "main"},
		struct{ path, branch string }{"/repo/.worktrees/feature-task-a", "feature/task-a"},
		struct{ path, branch string }{"/repo/.worktrees/feature-task-b", "feature/task-b"},
	)

	r := newSyncMockRunner(porcelain, syncMockConfig{noUpstream: true}, nil, nil)
	hctx := testContext(r)
	handler := handleSync(hctx)

	result := callTool(t, handler, map[string]any{"all": true})
	data := unmarshalJSON[syncResult](t, result)

	if len(data.Results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(data.Results))
	}
	for _, sr := range data.Results {
		if !sr.Synced {
			t.Errorf("expected %s to be synced", sr.Branch)
		}
		if !sr.PushSkipped {
			t.Errorf("expected %s push to be skipped (no upstream)", sr.Branch)
		}
	}
}

func TestSyncSingleDirtySkipped(t *testing.T) {
	porcelain := worktreePorcelain(
		struct{ path, branch string }{"/repo", "main"},
		struct{ path, branch string }{"/repo/.worktrees/feature-my-task", "feature/my-task"},
	)

	r := newSyncMockRunner(porcelain, syncMockConfig{dirty: true}, nil, nil)
	hctx := testContext(r)
	handler := handleSync(hctx)

	result := callTool(t, handler, map[string]any{"task": "my-task"})
	data := unmarshalJSON[syncResult](t, result)

	if len(data.Results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(data.Results))
	}
	sr := data.Results[0]
	if !sr.Skipped {
		t.Errorf("expected skipped=true for dirty worktree")
	}
	if sr.SkipReason != "dirty" {
		t.Errorf("expected skip_reason='dirty', got %q", sr.SkipReason)
	}
}

func TestSyncSingleRebaseFails(t *testing.T) {
	porcelain := worktreePorcelain(
		struct{ path, branch string }{"/repo", "main"},
		struct{ path, branch string }{"/repo/.worktrees/feature-my-task", "feature/my-task"},
	)

	r := newSyncMockRunner(porcelain, syncMockConfig{rebaseFails: true}, nil, nil)
	hctx := testContext(r)
	handler := handleSync(hctx)

	result := callTool(t, handler, map[string]any{"task": "my-task"})
	data := unmarshalJSON[syncResult](t, result)

	if len(data.Results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(data.Results))
	}
	sr := data.Results[0]
	if !sr.Failed {
		t.Errorf("expected failed=true")
	}
	if sr.FailureHint == "" {
		t.Errorf("expected failure_hint to be set")
	}
	if !strings.Contains(sr.FailureHint, rebaseCmd) {
		t.Errorf("expected failure_hint to contain 'rebase', got %q", sr.FailureHint)
	}
}

func TestSyncSinglePushFails(t *testing.T) {
	porcelain := worktreePorcelain(
		struct{ path, branch string }{"/repo", "main"},
		struct{ path, branch string }{"/repo/.worktrees/feature-my-task", "feature/my-task"},
	)

	r := newSyncMockRunner(porcelain, syncMockConfig{pushFails: true}, nil, nil)
	hctx := testContext(r)
	handler := handleSync(hctx)

	result := callTool(t, handler, map[string]any{"task": "my-task"})
	data := unmarshalJSON[syncResult](t, result)

	if len(data.Results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(data.Results))
	}
	sr := data.Results[0]
	if !sr.Synced {
		t.Errorf("expected synced=true (rebase succeeded)")
	}
	if !sr.PushFailed {
		t.Errorf("expected push_failed=true")
	}
	if sr.PushError == "" {
		t.Errorf("expected push_error to be set")
	}
}

func TestSyncListWorktreesFails(t *testing.T) {
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			if len(args) > 0 && args[0] == gitFetch {
				return "", nil
			}
			if len(args) >= 2 && args[0] == gitWorktree && args[1] == gitList {
				return "", errors.New("worktree list failed")
			}
			return "", nil
		},
	}
	hctx := testContext(r)
	handler := handleSync(hctx)

	result := callTool(t, handler, map[string]any{"task": "my-task"})
	errText := resultError(t, result)
	if !strings.Contains(errText, "worktree list failed") {
		t.Errorf("expected worktree list error, got: %s", errText)
	}
}

func TestSyncListWorktreesFailsAll(t *testing.T) {
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			if len(args) > 0 && args[0] == gitFetch {
				return "", nil
			}
			if len(args) >= 2 && args[0] == gitWorktree && args[1] == gitList {
				return "", errors.New("worktree list failed")
			}
			return "", nil
		},
	}
	hctx := testContext(r)
	handler := handleSync(hctx)

	result := callTool(t, handler, map[string]any{"all": true})
	errText := resultError(t, result)
	if !strings.Contains(errText, "worktree list failed") {
		t.Errorf("expected worktree list error, got: %s", errText)
	}
}

func TestConvertSyncResult(t *testing.T) {
	sr := operations.SyncWorktreeResult{
		Branch:      "feature/test",
		Synced:      true,
		Skipped:     false,
		SkipReason:  "",
		Failed:      false,
		FailureHint: "",
		Pushed:      true,
		PushSkipped: false,
		PushFailed:  false,
		PushError:   "",
	}

	result := convertSyncResult(sr)

	if result.Branch != "feature/test" {
		t.Errorf("expected branch 'feature/test', got %q", result.Branch)
	}
	if !result.Synced {
		t.Errorf("expected synced=true")
	}
	if result.Skipped {
		t.Errorf("expected skipped=false")
	}
	if !result.Pushed {
		t.Errorf("expected pushed=true")
	}
}

func TestConvertSyncResultFailure(t *testing.T) {
	sr := operations.SyncWorktreeResult{
		Branch:      "feature/broken",
		Synced:      false,
		Failed:      true,
		FailureHint: "cd /path && git rebase main",
		PushFailed:  true,
		PushError:   "push error",
	}

	result := convertSyncResult(sr)

	if result.Branch != "feature/broken" {
		t.Errorf("expected branch 'feature/broken', got %q", result.Branch)
	}
	if result.Synced {
		t.Errorf("expected synced=false")
	}
	if !result.Failed {
		t.Errorf("expected failed=true")
	}
	if result.FailureHint != "cd /path && git rebase main" {
		t.Errorf("expected failure_hint, got %q", result.FailureHint)
	}
	if !result.PushFailed {
		t.Errorf("expected push_failed=true")
	}
	if result.PushError != "push error" {
		t.Errorf("expected push_error, got %q", result.PushError)
	}
}

func TestConvertSyncResultSkipped(t *testing.T) {
	sr := operations.SyncWorktreeResult{
		Branch:     "feature/dirty",
		Skipped:    true,
		SkipReason: "dirty",
	}

	result := convertSyncResult(sr)

	if !result.Skipped {
		t.Errorf("expected skipped=true")
	}
	if result.SkipReason != "dirty" {
		t.Errorf("expected skip_reason='dirty', got %q", result.SkipReason)
	}
}

func TestSyncMultipleNoEligible(t *testing.T) {
	porcelain := worktreePorcelain(
		struct{ path, branch string }{"/repo", "main"},
	)

	r := newSyncMockRunner(porcelain, syncMockConfig{}, nil, nil)
	hctx := testContext(r)
	handler := handleSync(hctx)

	result := callTool(t, handler, map[string]any{"all": true})
	data := unmarshalJSON[syncResult](t, result)

	if len(data.Results) != 0 {
		t.Errorf("expected 0 results for no eligible worktrees, got %d", len(data.Results))
	}
}

func TestSyncSingleStatusCheckError(t *testing.T) {
	porcelain := worktreePorcelain(
		struct{ path, branch string }{"/repo", "main"},
		struct{ path, branch string }{"/repo/.worktrees/feature-my-task", "feature/my-task"},
	)

	r := newSyncMockRunner(porcelain, syncMockConfig{statusError: true}, nil, nil)
	hctx := testContext(r)
	handler := handleSync(hctx)

	result := callTool(t, handler, map[string]any{"task": "my-task"})
	data := unmarshalJSON[syncResult](t, result)

	if len(data.Results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(data.Results))
	}
	sr := data.Results[0]
	if !sr.Skipped {
		t.Errorf("expected skipped=true when status check fails")
	}
	if !strings.Contains(sr.SkipReason, "could not check status") {
		t.Errorf("expected skip reason about status check, got %q", sr.SkipReason)
	}
}

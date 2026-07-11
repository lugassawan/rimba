package mcp

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/lugassawan/rimba/internal/executor"
	"github.com/lugassawan/rimba/internal/resolver"
)

const cmdEchoTest = "echo test"

func TestExecToolRequiresCommand(t *testing.T) {
	hctx := testContext(&mockRunner{})
	handler := handleExec(hctx)

	result := callTool(t, handler, map[string]any{"all": true})
	errText := resultError(t, result)
	if !strings.Contains(errText, "command is required") {
		t.Errorf("expected 'command is required' error, got: %s", errText)
	}
}

func TestExecToolRequiresSelector(t *testing.T) {
	hctx := testContext(&mockRunner{})
	handler := handleExec(hctx)

	result := callTool(t, handler, map[string]any{"command": "echo hello"})
	errText := resultError(t, result)
	if !strings.Contains(errText, "all=true or type") {
		t.Errorf("expected selector error, got: %s", errText)
	}
}

func TestExecToolInvalidType(t *testing.T) {
	hctx := testContext(&mockRunner{})
	handler := handleExec(hctx)

	result := callTool(t, handler, map[string]any{"command": "echo", "type": "invalid"})
	errText := resultError(t, result)
	if !strings.Contains(errText, "invalid type") {
		t.Errorf("expected 'invalid type' error, got: %s", errText)
	}
}

func TestExecToolNoWorktrees(t *testing.T) {
	porcelain := worktreePorcelain(
		struct{ path, branch string }{"/repo", "main"},
	)

	r := &mockRunner{
		run: func(args ...string) (string, error) {
			return porcelain, nil
		},
		runInDir: func(dir string, args ...string) (string, error) {
			return "", nil
		},
	}
	hctx := testContext(r)
	handler := handleExec(hctx)

	result := callTool(t, handler, map[string]any{"command": "echo hi", "all": true})
	data := unmarshalJSON[execData](t, result)
	if !data.Success {
		t.Error("expected success=true for empty result set")
	}
	if len(data.Results) != 0 {
		t.Errorf("expected 0 results, got %d", len(data.Results))
	}
}

func TestExecToolRequiresConfig(t *testing.T) {
	hctx := &HandlerContext{
		Runner:   &mockRunner{},
		Config:   nil,
		RepoRoot: "/repo",
		Version:  "test",
	}
	handler := handleExec(hctx)

	result := callTool(t, handler, map[string]any{"command": "echo", "all": true})
	errText := resultError(t, result)
	if !strings.Contains(errText, "not initialized") {
		t.Errorf("expected config error, got: %s", errText)
	}
}

func TestBuildExecDataAllSuccess(t *testing.T) {
	results := []executor.Result{
		{Target: executor.Target{Path: "/wt/a", Branch: "feature/a", Task: "a"}, ExitCode: 0, Stdout: []byte("ok\n")},
		{Target: executor.Target{Path: "/wt/b", Branch: "feature/b", Task: "b"}, ExitCode: 0, Stdout: []byte("done\n")},
	}
	data := buildExecData(cmdEchoTest, results)
	if !data.Success {
		t.Error("expected success=true")
	}
	if len(data.Results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(data.Results))
	}
	if data.Results[0].Stdout != "ok\n" {
		t.Errorf("stdout = %q", data.Results[0].Stdout)
	}
	if data.Command != cmdEchoTest {
		t.Errorf("command = %q", data.Command)
	}
}

func TestBuildExecDataWithFailure(t *testing.T) {
	results := []executor.Result{
		{Target: executor.Target{Task: "a"}, ExitCode: 0},
		{Target: executor.Target{Task: "b"}, ExitCode: 1, Stderr: []byte("fail\n")},
	}
	data := buildExecData("test", results)
	if data.Success {
		t.Error("expected success=false")
	}
	if data.Results[1].ExitCode != 1 {
		t.Errorf("exit_code = %d", data.Results[1].ExitCode)
	}
	if data.Results[1].Stderr != "fail\n" {
		t.Errorf("stderr = %q", data.Results[1].Stderr)
	}
}

func TestBuildExecDataWithError(t *testing.T) {
	results := []executor.Result{
		{Target: executor.Target{Task: "a"}, ExitCode: 0, Err: errors.New("exec error")},
	}
	data := buildExecData("cmd", results)
	if data.Success {
		t.Error("expected success=false")
	}
	if data.Results[0].Error != "exec error" {
		t.Errorf("error = %q", data.Results[0].Error)
	}
}

func TestBuildExecDataWithCancelled(t *testing.T) {
	results := []executor.Result{
		{Target: executor.Target{Task: "a"}, ExitCode: 0, Cancelled: true},
	}
	data := buildExecData("cmd", results)
	if !data.Results[0].Cancelled {
		t.Error("expected cancelled=true")
	}
}

func TestBuildExecDataEmpty(t *testing.T) {
	data := buildExecData("cmd", nil)
	if !data.Success {
		t.Error("expected success=true for empty")
	}
	if len(data.Results) != 0 {
		t.Errorf("expected 0 results, got %d", len(data.Results))
	}
}

func TestFilterDirty(t *testing.T) {
	worktrees := []resolver.WorktreeInfo{
		{Path: "/wt/a", Branch: "feature/a"},
		{Path: "/wt/b", Branch: "feature/b"},
		{Path: "/wt/c", Branch: "feature/c"},
	}
	r := &mockRunner{
		runInDir: func(dir string, args ...string) (string, error) {
			if dir == "/wt/b" {
				return " M file.go\n", nil // dirty
			}
			return "", nil // clean
		},
	}
	result := filterDirty(context.Background(), r, worktrees)
	if len(result) != 1 {
		t.Fatalf("expected 1 dirty, got %d", len(result))
	}
	if result[0].Path != "/wt/b" {
		t.Errorf("path = %q, want /wt/b", result[0].Path)
	}
}

func TestFilterDirtyAll(t *testing.T) {
	worktrees := []resolver.WorktreeInfo{
		{Path: "/wt/a", Branch: "feature/a"},
	}
	r := &mockRunner{
		runInDir: func(dir string, args ...string) (string, error) {
			return dirtyOutput, nil
		},
	}
	result := filterDirty(context.Background(), r, worktrees)
	if len(result) != 1 {
		t.Fatalf("expected 1, got %d", len(result))
	}
}

func TestFilterDirtyNone(t *testing.T) {
	worktrees := []resolver.WorktreeInfo{
		{Path: "/wt/a", Branch: "feature/a"},
	}
	r := &mockRunner{
		runInDir: func(dir string, args ...string) (string, error) {
			return "", nil
		},
	}
	result := filterDirty(context.Background(), r, worktrees)
	if len(result) != 0 {
		t.Fatalf("expected 0, got %d", len(result))
	}
}

func TestFilterDirtyError(t *testing.T) {
	worktrees := []resolver.WorktreeInfo{
		{Path: "/wt/a", Branch: "feature/a"},
	}
	r := &mockRunner{
		runInDir: func(dir string, args ...string) (string, error) {
			return "", errors.New("git error")
		},
	}
	origStderr := os.Stderr
	pr, pw, _ := os.Pipe()
	os.Stderr = pw

	result := filterDirty(context.Background(), r, worktrees)

	pw.Close()
	os.Stderr = origStderr
	_, _ = io.Copy(io.Discard, pr)

	if len(result) != 1 {
		t.Fatalf("expected erroring worktree treated as dirty (len 1), got %d", len(result))
	}
}

func TestFilterDirtyErrorIncludedAndWarned(t *testing.T) {
	worktrees := []resolver.WorktreeInfo{
		{Path: "/wt/a", Branch: "feature/a"},
	}
	r := &mockRunner{
		runInDir: func(dir string, args ...string) (string, error) {
			return "", errors.New("permission denied")
		},
	}

	origStderr := os.Stderr
	pr, pw, _ := os.Pipe()
	os.Stderr = pw

	result := filterDirty(context.Background(), r, worktrees)

	pw.Close()
	os.Stderr = origStderr
	var stderrBuf strings.Builder
	_, _ = io.Copy(&stderrBuf, pr)

	if len(result) != 1 {
		t.Fatalf("expected erroring worktree to be included, got %d", len(result))
	}
	if result[0].Path != "/wt/a" {
		t.Errorf("path = %q, want /wt/a", result[0].Path)
	}
	if !strings.Contains(stderrBuf.String(), "Warning: cannot check dirty status") {
		t.Errorf("stderr = %q, want warning", stderrBuf.String())
	}
}

// blockUntilCancelRunner blocks each RunInDir call on ctx.Done(), closing
// barrierCh once barrierN calls are in flight so a test can cancel deterministically.
type blockUntilCancelRunner struct {
	started     atomic.Int32
	barrierN    int32
	barrierCh   chan struct{}
	barrierOnce sync.Once
}

func (r *blockUntilCancelRunner) Run(_ context.Context, _ ...string) (string, error) {
	return "", nil
}

func (r *blockUntilCancelRunner) RunInDir(ctx context.Context, _ string, _ ...string) (string, error) {
	if r.started.Add(1) == r.barrierN {
		r.barrierOnce.Do(func() { close(r.barrierCh) })
	}
	<-ctx.Done()
	return "", ctx.Err()
}

// TestFilterDirtyIncludesUncheckedWorktreesOnCancel proves that queued
// worktrees a whole-context cancel prevents from ever running are fail-safe
// included, not silently dropped as "clean" via dirtyCheckResult's zero value.
func TestFilterDirtyIncludesUncheckedWorktreesOnCancel(t *testing.T) {
	const n = 10 // > 8: filterDirty's concurrency is hard-coded to 8
	worktrees := make([]resolver.WorktreeInfo, n)
	for i := range worktrees {
		worktrees[i] = resolver.WorktreeInfo{Path: fmt.Sprintf("/wt/%d", i)}
	}

	r := &blockUntilCancelRunner{barrierN: 8, barrierCh: make(chan struct{})}
	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan []resolver.WorktreeInfo, 1)
	go func() { done <- filterDirty(ctx, r, worktrees) }()

	select {
	case <-r.barrierCh:
	case <-time.After(5 * time.Second):
		t.Fatal("first 8 workers never started")
	}
	cancel()

	var result []resolver.WorktreeInfo
	select {
	case result = <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("filterDirty did not return after cancel")
	}

	if len(result) != n {
		t.Fatalf("expected all %d worktrees included as a cancellation fail-safe, got %d", n, len(result))
	}
}

func TestExcludeOrphanedExecNoOpWhenNoCustomPrefix(t *testing.T) {
	worktrees := []resolver.WorktreeInfo{
		{Branch: "feature/foo"},
		{Branch: "some-orphan-looking-branch"},
	}

	got := excludeOrphanedExec(worktrees, resolver.DefaultPrefixSet(), "main")

	if len(got) != len(worktrees) {
		t.Errorf("excludeOrphanedExec() with no custom prefix = %d worktrees, want all %d kept (no-op)", len(got), len(worktrees))
	}
}

func TestExcludeOrphanedExecExcludesAndWarns(t *testing.T) {
	ps := resolver.NewPrefixSet([]resolver.PrefixSpec{{Prefix: "TASK-"}})
	worktrees := []resolver.WorktreeInfo{
		{Branch: "TASK-123"},
		{Branch: "PROJ-456"}, // orphaned: PROJ- is not configured
	}

	origStderr := os.Stderr
	pr, pw, _ := os.Pipe()
	os.Stderr = pw

	got := excludeOrphanedExec(worktrees, ps, "main")

	pw.Close()
	os.Stderr = origStderr
	var stderrBuf strings.Builder
	_, _ = io.Copy(&stderrBuf, pr)

	if len(got) != 1 || got[0].Branch != "TASK-123" {
		t.Errorf("excludeOrphanedExec() = %+v, want only the TASK-123 worktree kept", got)
	}
	if !strings.Contains(stderrBuf.String(), "excluding 1 worktree") {
		t.Errorf("excludeOrphanedExec() stderr = %q, want an exclusion warning", stderrBuf.String())
	}
}

func TestExcludeOrphanedExecKeepsAllWhenNoneOrphaned(t *testing.T) {
	ps := resolver.NewPrefixSet([]resolver.PrefixSpec{{Prefix: "PROJ-"}})
	worktrees := []resolver.WorktreeInfo{
		{Branch: "PROJ-1"},
		{Branch: "PROJ-2"},
	}

	got := excludeOrphanedExec(worktrees, ps, "main")

	if len(got) != 2 {
		t.Errorf("excludeOrphanedExec() = %+v, want both worktrees kept", got)
	}
}

func TestExecToolWithTypeFilter(t *testing.T) {
	porcelain := worktreePorcelain(
		struct{ path, branch string }{"/repo", "main"},
		struct{ path, branch string }{"/repo/.worktrees/feature-a", "feature/a"},
		struct{ path, branch string }{"/repo/.worktrees/bugfix-b", "bugfix/b"},
	)
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			return porcelain, nil
		},
		runInDir: func(dir string, args ...string) (string, error) {
			return "", nil
		},
	}
	hctx := testContext(r)
	handler := handleExec(hctx)

	// type=bugfix with no real executor won't run actual commands in unit test,
	// but verifying the filtering path works correctly.
	result := callTool(t, handler, map[string]any{"command": cmdEchoTest, "type": "bugfix"})
	data := unmarshalJSON[execData](t, result)
	if data.Command != cmdEchoTest {
		t.Errorf("command = %q", data.Command)
	}
}

func TestExecToolListError(t *testing.T) {
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			if len(args) > 0 && args[0] == gitWorktree {
				return "", errors.New("git error")
			}
			return "", nil
		},
	}
	hctx := testContext(r)
	handler := handleExec(hctx)

	result := callTool(t, handler, map[string]any{"command": "echo", "all": true})
	if !result.IsError {
		t.Error("expected error")
	}
}

func TestExecToolWithDirtyFilter(t *testing.T) {
	porcelain := worktreePorcelain(
		struct{ path, branch string }{"/repo", "main"},
		struct{ path, branch string }{"/wt/feature-a", "feature/a"},
		struct{ path, branch string }{"/wt/feature-b", "feature/b"},
	)
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			return porcelain, nil
		},
		runInDir: func(dir string, args ...string) (string, error) {
			// Only feature-a is dirty
			if dir == "/wt/feature-a" && len(args) > 0 && args[0] == gitStatus {
				return " M file.go\n", nil
			}
			return "", nil
		},
	}
	hctx := testContext(r)
	handler := handleExec(hctx)

	// all=true with dirty=true should filter then execute
	result := callTool(t, handler, map[string]any{"command": cmdEchoTest, "all": true, "dirty": true})
	data := unmarshalJSON[execData](t, result)
	if data.Command != cmdEchoTest {
		t.Errorf("command = %q", data.Command)
	}
}

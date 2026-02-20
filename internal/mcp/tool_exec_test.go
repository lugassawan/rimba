package mcp

import (
	"errors"
	"strings"
	"testing"

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
	result := filterDirty(r, worktrees)
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
	result := filterDirty(r, worktrees)
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
	result := filterDirty(r, worktrees)
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
	result := filterDirty(r, worktrees)
	if len(result) != 0 {
		t.Fatalf("expected 0 on error, got %d", len(result))
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

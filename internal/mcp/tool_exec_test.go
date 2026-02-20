package mcp

import (
	"strings"
	"testing"
)

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

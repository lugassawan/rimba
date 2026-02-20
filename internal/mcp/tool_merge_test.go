package mcp

import (
	"strings"
	"testing"
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

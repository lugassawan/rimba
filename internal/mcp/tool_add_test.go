package mcp

import (
	"strings"
	"testing"
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

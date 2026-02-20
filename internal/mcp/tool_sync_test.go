package mcp

import (
	"strings"
	"testing"
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
			// Fetch fails (no remote) — that's fine
			if len(args) > 0 && args[0] == "fetch" {
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

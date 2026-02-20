package mcp

import (
	"strings"
	"testing"
)

func TestCleanToolRequiresMode(t *testing.T) {
	hctx := testContext(&mockRunner{})
	handler := handleClean(hctx)

	result := callTool(t, handler, nil)
	errText := resultError(t, result)
	if !strings.Contains(errText, "mode is required") {
		t.Errorf("expected 'mode is required', got: %s", errText)
	}
}

func TestCleanToolInvalidMode(t *testing.T) {
	hctx := testContext(&mockRunner{})
	handler := handleClean(hctx)

	result := callTool(t, handler, map[string]any{"mode": "invalid"})
	errText := resultError(t, result)
	if !strings.Contains(errText, "invalid mode") {
		t.Errorf("expected 'invalid mode' error, got: %s", errText)
	}
}

func TestCleanToolPrune(t *testing.T) {
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			return "", nil
		},
	}
	hctx := testContext(r)
	handler := handleClean(hctx)

	result := callTool(t, handler, map[string]any{"mode": "prune"})
	data := unmarshalJSON[cleanResult](t, result)
	if data.Mode != "prune" {
		t.Errorf("mode = %q, want 'prune'", data.Mode)
	}
}

func TestCleanToolPruneDryRun(t *testing.T) {
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			return "", nil
		},
	}
	hctx := testContext(r)
	handler := handleClean(hctx)

	result := callTool(t, handler, map[string]any{"mode": "prune", "dry_run": true})
	data := unmarshalJSON[cleanResult](t, result)
	if !data.DryRun {
		t.Error("expected dry_run=true")
	}
}

func TestCleanToolMergedNoWorktrees(t *testing.T) {
	porcelain := worktreePorcelain(
		struct{ path, branch string }{"/repo", "main"},
	)

	r := &mockRunner{
		run: func(args ...string) (string, error) {
			if len(args) > 0 && args[0] == "fetch" {
				return "", nil
			}
			if len(args) > 0 && args[0] == "branch" {
				return "", nil // no merged branches
			}
			return porcelain, nil
		},
	}
	hctx := testContext(r)
	handler := handleClean(hctx)

	result := callTool(t, handler, map[string]any{"mode": "merged"})
	data := unmarshalJSON[cleanResult](t, result)
	if data.Mode != "merged" {
		t.Errorf("mode = %q, want 'merged'", data.Mode)
	}
	if len(data.Removed) != 0 {
		t.Errorf("expected 0 removed, got %d", len(data.Removed))
	}
}

func TestCleanToolStaleNoWorktrees(t *testing.T) {
	porcelain := worktreePorcelain(
		struct{ path, branch string }{"/repo", "main"},
	)

	r := &mockRunner{
		run: func(args ...string) (string, error) {
			return porcelain, nil
		},
	}
	hctx := testContext(r)
	handler := handleClean(hctx)

	result := callTool(t, handler, map[string]any{"mode": "stale"})
	data := unmarshalJSON[cleanResult](t, result)
	if data.Mode != "stale" {
		t.Errorf("mode = %q, want 'stale'", data.Mode)
	}
	if len(data.Removed) != 0 {
		t.Errorf("expected 0 removed, got %d", len(data.Removed))
	}
}

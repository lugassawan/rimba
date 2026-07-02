package mcp

import (
	"context"
	"errors"
	"strings"
	"testing"

	mcplib "github.com/mark3labs/mcp-go/mcp"

	"github.com/lugassawan/rimba/internal/errhint"
)

// TestInlineValidationHints verifies that every MCP handler returns a "To fix:" hint
// when its own inline validation fires (before any git runner call).
func TestInlineValidationHints(t *testing.T) {
	hctx := testContext(&mockRunner{})

	tests := []struct {
		name    string
		handler func(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error)
		args    map[string]any
		wantErr string
	}{
		{
			name:    "add: task is required",
			handler: handleAdd(hctx),
			args:    nil,
			wantErr: "task is required",
		},
		{
			name:    "add: invalid type",
			handler: handleAdd(hctx),
			args:    map[string]any{"task": "foo", "type": "invalid-xyz"},
			wantErr: "invalid type",
		},
		{
			name:    "clean: mode is required",
			handler: handleClean(hctx),
			args:    nil,
			wantErr: "mode is required",
		},
		{
			name:    "clean: invalid mode",
			handler: handleClean(hctx),
			args:    map[string]any{"mode": "bogus"},
			wantErr: "invalid mode",
		},
		{
			name:    "exec: command is required",
			handler: handleExec(hctx),
			args:    nil,
			wantErr: "command is required",
		},
		{
			name:    "exec: provide all or type",
			handler: handleExec(hctx),
			args:    map[string]any{"command": "echo hi"},
			wantErr: "provide all=true or type",
		},
		{
			name:    "exec: invalid type",
			handler: handleExec(hctx),
			args:    map[string]any{"command": "echo hi", "type": "invalid-xyz"},
			wantErr: "invalid type",
		},
		{
			name:    "list: invalid type",
			handler: handleList(hctx),
			args:    map[string]any{"type": "invalid-xyz"},
			wantErr: "invalid type",
		},
		{
			name:    "merge: source is required",
			handler: handleMerge(hctx),
			args:    nil,
			wantErr: "source is required",
		},
		{
			name:    "remove: task is required",
			handler: handleRemove(hctx),
			args:    nil,
			wantErr: "task is required",
		},
		{
			name:    "sync: provide task or all",
			handler: handleSync(hctx),
			args:    nil,
			wantErr: "provide a task name or set all=true",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := callTool(t, tc.handler, tc.args)
			errText := resultError(t, result)
			if !strings.Contains(errText, "To fix:") {
				t.Errorf("expected 'To fix:' hint in error, got: %q", errText)
			}
			if !strings.Contains(errText, tc.wantErr) {
				t.Errorf("expected %q in error text, got: %q", tc.wantErr, errText)
			}
		})
	}
}

// TestSyncWorktreeNotFoundHint verifies that the "worktree not found" path in sync
// carries a "To fix:" hint. Requires a runner that provides a real worktree list.
func TestSyncWorktreeNotFoundHint(t *testing.T) {
	porcelain := worktreePorcelain(
		struct{ path, branch string }{"/repo", "main"},
	)
	r := &mockRunner{
		run: func(args ...string) (string, error) {
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
	if !strings.Contains(errText, "To fix:") {
		t.Errorf("expected 'To fix:' hint in error, got: %q", errText)
	}
	if !strings.Contains(errText, "nonexistent") {
		t.Errorf("expected task name in error, got: %q", errText)
	}
}

// TestPassThroughHintPreserved verifies that operation-layer errors already carrying
// an errhint "To fix:" suffix are not stripped when routed through errorResult.
func TestPassThroughHintPreserved(t *testing.T) {
	hintedErr := errhint.WithFix(errors.New("worktree list failed"), "run git worktree prune to repair refs")
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			return "", hintedErr
		},
	}
	hctx := testContext(r)
	handler := handleRemove(hctx)

	result := callTool(t, handler, map[string]any{"task": "my-task"})
	errText := resultError(t, result)
	if !strings.Contains(errText, "To fix:") {
		t.Errorf("expected pass-through 'To fix:' hint to be preserved, got: %q", errText)
	}
	if !strings.Contains(errText, "git worktree prune") {
		t.Errorf("expected hint text to be preserved, got: %q", errText)
	}
}

package mcp

import (
	"strings"
	"testing"
)

func TestConflictCheckToolRequiresConfig(t *testing.T) {
	hctx := &HandlerContext{
		Runner:   &mockRunner{},
		Config:   nil,
		RepoRoot: "/repo",
		Version:  "test",
	}
	handler := handleConflictCheck(hctx)

	result := callTool(t, handler, nil)
	errText := resultError(t, result)
	if !strings.Contains(errText, "not initialized") {
		t.Errorf("expected config error, got: %s", errText)
	}
}

func TestConflictCheckToolNoEligible(t *testing.T) {
	porcelain := worktreePorcelain(
		struct{ path, branch string }{"/repo", "main"},
	)

	r := &mockRunner{
		run: func(args ...string) (string, error) {
			return porcelain, nil
		},
	}
	hctx := testContext(r)
	handler := handleConflictCheck(hctx)

	result := callTool(t, handler, nil)
	data := unmarshalJSON[conflictCheckData](t, result)
	if len(data.Overlaps) != 0 {
		t.Errorf("expected 0 overlaps, got %d", len(data.Overlaps))
	}
}

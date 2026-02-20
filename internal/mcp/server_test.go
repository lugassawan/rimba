package mcp

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/lugassawan/rimba/internal/config"
	mcplib "github.com/mark3labs/mcp-go/mcp"
)

const (
	gitSymbolicRef = "symbolic-ref"
	refsOriginMain = "refs/remotes/origin/main"
	gitWorktree    = "worktree"
	gitRevParse    = "rev-parse"
	gitRevList     = "rev-list"
	gitStatus      = "status"
	gitList        = "list"
	revListEven    = "0\t0"
	dirtyOutput    = " M dirty.go\n"
)

// mockRunner implements git.Runner for unit tests.
type mockRunner struct {
	run      func(args ...string) (string, error)
	runInDir func(dir string, args ...string) (string, error)
}

func (m *mockRunner) Run(args ...string) (string, error) {
	if m.run == nil {
		return "", nil
	}
	return m.run(args...)
}

func (m *mockRunner) RunInDir(dir string, args ...string) (string, error) {
	if m.runInDir == nil {
		return "", nil
	}
	return m.runInDir(dir, args...)
}

// testConfig returns a minimal config suitable for testing.
func testConfig() *config.Config {
	return &config.Config{
		WorktreeDir:   ".worktrees",
		DefaultSource: "main",
		CopyFiles:     []string{".editorconfig"},
	}
}

// testContext returns a HandlerContext with a mock runner and test config.
func testContext(r *mockRunner) *HandlerContext {
	return &HandlerContext{
		Runner:   r,
		Config:   testConfig(),
		RepoRoot: "/repo",
		Version:  "test",
	}
}

// callTool is a test helper that invokes a tool handler with the given arguments.
func callTool(t *testing.T, handler func(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error), args map[string]any) *mcplib.CallToolResult {
	t.Helper()
	req := mcplib.CallToolRequest{}
	req.Params.Arguments = args
	result, err := handler(context.Background(), req)
	if err != nil {
		t.Fatalf("handler returned protocol error: %v", err)
	}
	return result
}

// resultJSON extracts the JSON text from a tool result.
func resultJSON(t *testing.T, result *mcplib.CallToolResult) string {
	t.Helper()
	if result.IsError {
		t.Fatalf("expected success result, got error: %v", result.Content)
	}
	tc, ok := mcplib.AsTextContent(result.Content[0])
	if !ok {
		t.Fatal("result content is not TextContent")
	}
	return tc.Text
}

// resultError extracts the error text from a tool result.
func resultError(t *testing.T, result *mcplib.CallToolResult) string {
	t.Helper()
	if !result.IsError {
		t.Fatal("expected error result, got success")
	}
	tc, ok := mcplib.AsTextContent(result.Content[0])
	if !ok {
		t.Fatal("error content is not TextContent")
	}
	return tc.Text
}

// unmarshalJSON is a helper to unmarshal JSON from a tool result into a target type.
func unmarshalJSON[T any](t *testing.T, result *mcplib.CallToolResult) T {
	t.Helper()
	jsonStr := resultJSON(t, result)
	var v T
	if err := json.Unmarshal([]byte(jsonStr), &v); err != nil {
		t.Fatalf("failed to unmarshal: %v\njson: %s", err, jsonStr)
	}
	return v
}

func TestNewServer(t *testing.T) {
	hctx := testContext(&mockRunner{})
	s := NewServer(hctx)

	tools := s.ListTools()
	expectedTools := []string{"list", "add", "remove", "status", "exec", "conflict-check", "merge", "sync", "clean"}
	for _, name := range expectedTools {
		if _, exists := tools[name]; !exists {
			t.Errorf("expected tool %q to be registered", name)
		}
	}

	if len(tools) != len(expectedTools) {
		t.Errorf("expected %d tools, got %d", len(expectedTools), len(tools))
	}
}

// worktreePorcelain generates git worktree --porcelain output.
func worktreePorcelain(entries ...struct{ path, branch string }) string {
	lines := make([]string, 0, 4*len(entries))
	for _, e := range entries {
		lines = append(lines,
			"worktree "+e.path,
			"HEAD abc123",
			"branch refs/heads/"+e.branch,
			"",
		)
	}
	return strings.Join(lines, "\n")
}

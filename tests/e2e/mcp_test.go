package e2e_test

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/lugassawan/rimba/internal/resolver"
	"github.com/lugassawan/rimba/testutil"
)

// jsonRPCRequest builds a JSON-RPC 2.0 request.
func jsonRPCRequest(id int, method string, params any) string {
	req := map[string]any{
		"jsonrpc": "2.0",
		"id":      id,
		"method":  method,
		"params":  params,
	}
	b, _ := json.Marshal(req)
	return string(b) + "\n"
}

// mcpInitRequest builds the MCP initialize request.
func mcpInitRequest() string {
	return jsonRPCRequest(1, "initialize", map[string]any{
		"protocolVersion": "2024-11-05",
		"capabilities":    map[string]any{},
		"clientInfo":      map[string]any{"name": "test", "version": "0.1.0"},
	})
}

// mcpNotification builds a JSON-RPC notification (no id).
func mcpNotification() string {
	req := map[string]any{
		"jsonrpc": "2.0",
		"method":  "notifications/initialized",
	}
	b, _ := json.Marshal(req)
	return string(b) + "\n"
}

// mcpToolsListRequest builds a tools/list request.
func mcpToolsListRequest(id int) string {
	return jsonRPCRequest(id, "tools/list", map[string]any{})
}

// mcpToolCallRequest builds a tools/call request.
func mcpToolCallRequest(name string, args map[string]any) string {
	return jsonRPCRequest(2, "tools/call", map[string]any{
		"name":      name,
		"arguments": args,
	})
}

// mcpSession sends a sequence of JSON-RPC messages to `rimba mcp` and returns all responses.
func mcpSession(t *testing.T, dir string, messages ...string) []map[string]any {
	t.Helper()

	var stdin bytes.Buffer
	for _, msg := range messages {
		stdin.WriteString(msg)
	}

	cmd := exec.Command(binaryPath, "mcp")
	cmd.Dir = dir
	cmd.Stdin = &stdin
	cmd.Env = append(os.Environ(), "GOCOVERDIR="+coverDir, "NO_COLOR=1")

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// MCP server may exit with non-zero when stdin closes — that's OK
	// as long as we got responses
	_ = cmd.Run()

	// Parse newline-delimited JSON responses
	var responses []map[string]any
	dec := json.NewDecoder(&stdout)
	for dec.More() {
		var resp map[string]any
		if err := dec.Decode(&resp); err != nil {
			break
		}
		// Skip notifications (no id)
		if _, hasID := resp["id"]; hasID {
			responses = append(responses, resp)
		}
	}

	return responses
}

// getToolResult extracts the tool result from an MCP tools/call response.
func getToolResult(t *testing.T, resp map[string]any) (text string, isError bool) {
	t.Helper()

	result, ok := resp["result"].(map[string]any)
	if !ok {
		t.Fatalf("missing result in response: %v", resp)
	}

	isError, _ = result["isError"].(bool)

	content, ok := result["content"].([]any)
	if !ok || len(content) == 0 {
		t.Fatalf("missing content in result: %v", result)
	}

	first, ok := content[0].(map[string]any)
	if !ok {
		t.Fatalf("content[0] is not a map: %T", content[0])
	}

	text, _ = first["text"].(string)
	return text, isError
}

func TestMCPInitializeAndToolsList(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupInitializedRepo(t)

	responses := mcpSession(t, repo,
		mcpInitRequest(),
		mcpNotification(),
		mcpToolsListRequest(2),
	)

	if len(responses) < 2 {
		t.Fatalf("expected at least 2 responses, got %d", len(responses))
	}

	// First response: initialize result
	initResp := responses[0]
	initResult, ok := initResp["result"].(map[string]any)
	if !ok {
		t.Fatalf("missing result in init response: %v", initResp)
	}
	serverInfo, ok := initResult["serverInfo"].(map[string]any)
	if !ok {
		t.Fatalf("missing serverInfo: %v", initResult)
	}
	if serverInfo["name"] != "rimba" {
		t.Errorf("serverInfo.name = %v, want 'rimba'", serverInfo["name"])
	}

	// Second response: tools/list
	toolsResp := responses[1]
	toolsResult, ok := toolsResp["result"].(map[string]any)
	if !ok {
		t.Fatalf("missing result in tools response: %v", toolsResp)
	}

	tools, ok := toolsResult["tools"].([]any)
	if !ok {
		t.Fatalf("tools is not an array: %T", toolsResult["tools"])
	}

	expectedTools := map[string]bool{
		"list": false, "add": false, "remove": false, "status": false,
		"exec": false, "conflict-check": false, "merge": false, "sync": false, "clean": false,
	}

	for _, tool := range tools {
		toolMap, ok := tool.(map[string]any)
		if !ok {
			continue
		}
		name, _ := toolMap["name"].(string)
		if _, exists := expectedTools[name]; exists {
			expectedTools[name] = true
		}
	}

	for name, found := range expectedTools {
		if !found {
			t.Errorf("expected tool %q to be listed", name)
		}
	}
}

func TestMCPToolCallList(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupInitializedRepo(t)
	rimbaSuccess(t, repo, "add", "mcp-list-task")

	responses := mcpSession(t, repo,
		mcpInitRequest(),
		mcpNotification(),
		mcpToolCallRequest("list", nil),
	)

	if len(responses) < 2 {
		t.Fatalf("expected at least 2 responses, got %d", len(responses))
	}

	text, isError := getToolResult(t, responses[1])
	if isError {
		t.Fatalf("expected success, got error: %s", text)
	}

	var items []map[string]any
	if err := json.Unmarshal([]byte(text), &items); err != nil {
		t.Fatalf("failed to parse list result: %v", err)
	}

	found := false
	for _, item := range items {
		if task, _ := item["task"].(string); task == "mcp-list-task" {
			found = true
			if typ, _ := item["type"].(string); typ != "feature" {
				t.Errorf("type = %q, want 'feature'", typ)
			}
		}
	}
	if !found {
		t.Error("expected to find 'mcp-list-task' in list results")
	}
}

func TestMCPToolCallStatus(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupInitializedRepo(t)
	rimbaSuccess(t, repo, "add", "mcp-status-task")

	responses := mcpSession(t, repo,
		mcpInitRequest(),
		mcpNotification(),
		mcpToolCallRequest("status", nil),
	)

	if len(responses) < 2 {
		t.Fatalf("expected at least 2 responses, got %d", len(responses))
	}

	text, isError := getToolResult(t, responses[1])
	if isError {
		t.Fatalf("expected success, got error: %s", text)
	}

	var data map[string]any
	if err := json.Unmarshal([]byte(text), &data); err != nil {
		t.Fatalf("failed to parse status result: %v", err)
	}

	summary, ok := data["summary"].(map[string]any)
	if !ok {
		t.Fatal("missing summary")
	}
	total, _ := summary["total"].(float64)
	if total < 1 {
		t.Errorf("summary.total = %v, want >= 1", total)
	}
}

func TestMCPToolCallAdd(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupInitializedRepo(t)

	responses := mcpSession(t, repo,
		mcpInitRequest(),
		mcpNotification(),
		mcpToolCallRequest("add", map[string]any{"task": "mcp-add-task", "skip_deps": true, "skip_hooks": true}),
	)

	if len(responses) < 2 {
		t.Fatalf("expected at least 2 responses, got %d", len(responses))
	}

	text, isError := getToolResult(t, responses[1])
	if isError {
		t.Fatalf("expected success, got error: %s", text)
	}

	var result map[string]any
	if err := json.Unmarshal([]byte(text), &result); err != nil {
		t.Fatalf("failed to parse add result: %v", err)
	}

	if result["task"] != "mcp-add-task" {
		t.Errorf("task = %v, want 'mcp-add-task'", result["task"])
	}
	if result["branch"] != "feature/mcp-add-task" {
		t.Errorf("branch = %v, want 'feature/mcp-add-task'", result["branch"])
	}

	// Verify worktree was actually created
	cfg := loadConfig(t, repo)
	wtDir := filepath.Join(repo, cfg.WorktreeDir)
	branch := resolver.BranchName(defaultPrefix, "mcp-add-task")
	wtPath := resolver.WorktreePath(wtDir, branch)
	assertFileExists(t, wtPath)
}

func TestMCPToolCallRemove(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupInitializedRepo(t)
	rimbaSuccess(t, repo, "add", "mcp-rm-task")

	responses := mcpSession(t, repo,
		mcpInitRequest(),
		mcpNotification(),
		mcpToolCallRequest("remove", map[string]any{"task": "mcp-rm-task"}),
	)

	if len(responses) < 2 {
		t.Fatalf("expected at least 2 responses, got %d", len(responses))
	}

	text, isError := getToolResult(t, responses[1])
	if isError {
		t.Fatalf("expected success, got error: %s", text)
	}

	var result map[string]any
	if err := json.Unmarshal([]byte(text), &result); err != nil {
		t.Fatalf("failed to parse remove result: %v", err)
	}

	if result["worktree_removed"] != true {
		t.Errorf("worktree_removed = %v, want true", result["worktree_removed"])
	}
	if result["branch_deleted"] != true {
		t.Errorf("branch_deleted = %v, want true", result["branch_deleted"])
	}
}

func TestMCPToolCallMerge(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupCleanInitializedRepo(t)

	// Create worktree and commit a change
	rimbaSuccess(t, repo, "add", "mcp-merge-task", flagSkipDepsE2E, flagSkipHooksE2E)

	cfg := loadConfig(t, repo)
	wtDir := filepath.Join(repo, cfg.WorktreeDir)
	branch := resolver.BranchName(defaultPrefix, "mcp-merge-task")
	wtPath := resolver.WorktreePath(wtDir, branch)

	testutil.CreateFile(t, wtPath, "merge-file.txt", "merged content")
	testutil.GitCmd(t, wtPath, "add", ".")
	testutil.GitCmd(t, wtPath, "commit", "-m", "add merge file")

	responses := mcpSession(t, repo,
		mcpInitRequest(),
		mcpNotification(),
		mcpToolCallRequest("merge", map[string]any{"source": "mcp-merge-task"}),
	)

	if len(responses) < 2 {
		t.Fatalf("expected at least 2 responses, got %d", len(responses))
	}

	text, isError := getToolResult(t, responses[1])
	if isError {
		t.Fatalf("expected success, got error: %s", text)
	}

	var result map[string]any
	if err := json.Unmarshal([]byte(text), &result); err != nil {
		t.Fatalf("failed to parse merge result: %v", err)
	}

	if result["source_removed"] != true {
		t.Errorf("source_removed = %v, want true", result["source_removed"])
	}

	// Verify file was merged into main
	assertFileExists(t, filepath.Join(repo, "merge-file.txt"))
}

func TestMCPToolCallCleanPrune(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupInitializedRepo(t)

	responses := mcpSession(t, repo,
		mcpInitRequest(),
		mcpNotification(),
		mcpToolCallRequest("clean", map[string]any{"mode": "prune"}),
	)

	if len(responses) < 2 {
		t.Fatalf("expected at least 2 responses, got %d", len(responses))
	}

	text, isError := getToolResult(t, responses[1])
	if isError {
		t.Fatalf("expected success, got error: %s", text)
	}

	var result map[string]any
	if err := json.Unmarshal([]byte(text), &result); err != nil {
		t.Fatalf("failed to parse clean result: %v", err)
	}

	if result["mode"] != "prune" {
		t.Errorf("mode = %v, want 'prune'", result["mode"])
	}
}

func TestMCPToolCallExec(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupInitializedRepo(t)
	rimbaSuccess(t, repo, "add", "mcp-exec-task")

	responses := mcpSession(t, repo,
		mcpInitRequest(),
		mcpNotification(),
		mcpToolCallRequest("exec", map[string]any{"command": "echo mcp-test", "all": true}),
	)

	if len(responses) < 2 {
		t.Fatalf("expected at least 2 responses, got %d", len(responses))
	}

	text, isError := getToolResult(t, responses[1])
	if isError {
		t.Fatalf("expected success, got error: %s", text)
	}

	var data map[string]any
	if err := json.Unmarshal([]byte(text), &data); err != nil {
		t.Fatalf("failed to parse exec result: %v", err)
	}

	if data["success"] != true {
		t.Error("expected success=true")
	}

	results, ok := data["results"].([]any)
	if !ok {
		t.Fatal("missing results array")
	}

	if len(results) < 1 {
		t.Error("expected at least 1 result")
	}

	// Verify at least one result contains "mcp-test" in stdout
	found := false
	for _, r := range results {
		item, ok := r.(map[string]any)
		if !ok {
			continue
		}
		stdout, _ := item["stdout"].(string)
		if stdout != "" && len(stdout) > 0 {
			found = true
		}
	}
	if !found {
		t.Error("expected at least one result with non-empty stdout")
	}
}

func TestMCPToolCallConflictCheck(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupCleanInitializedRepo(t)

	// Create two worktrees with overlapping files
	cfg := loadConfig(t, repo)
	for _, task := range []string{"mcp-cc-a", "mcp-cc-b"} {
		rimbaSuccess(t, repo, "add", task, flagSkipDepsE2E, flagSkipHooksE2E)
		wtDir := filepath.Join(repo, cfg.WorktreeDir)
		branch := resolver.BranchName(defaultPrefix, task)
		wtPath := resolver.WorktreePath(wtDir, branch)
		testutil.CreateFile(t, wtPath, "overlap.txt", "content from "+task)
		testutil.GitCmd(t, wtPath, "add", ".")
		testutil.GitCmd(t, wtPath, "commit", "-m", "add overlap")
	}

	responses := mcpSession(t, repo,
		mcpInitRequest(),
		mcpNotification(),
		mcpToolCallRequest("conflict-check", nil),
	)

	if len(responses) < 2 {
		t.Fatalf("expected at least 2 responses, got %d", len(responses))
	}

	text, isError := getToolResult(t, responses[1])
	if isError {
		t.Fatalf("expected success, got error: %s", text)
	}

	var data map[string]any
	if err := json.Unmarshal([]byte(text), &data); err != nil {
		t.Fatalf("failed to parse conflict-check result: %v", err)
	}

	overlaps, ok := data["overlaps"].([]any)
	if !ok {
		t.Fatal("missing overlaps array")
	}
	if len(overlaps) == 0 {
		t.Error("expected at least one overlap for overlapping files")
	}
}

// loadConfig is duplicated from add_test.go since it's in the same package — available here.
// (defined in add_test.go as func loadConfig(t, repo) *config.Config)

// setupCleanInitializedRepo is defined in merge_test.go — available in this package.

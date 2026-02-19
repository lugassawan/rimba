package e2e_test

import (
	"encoding/json"
	"strings"
	"testing"
)

// parseEnvelope parses a JSON envelope from stdout and returns the data field.
func parseEnvelope(t *testing.T, stdout string) map[string]any {
	t.Helper()
	var env map[string]any
	if err := json.Unmarshal([]byte(stdout), &env); err != nil {
		t.Fatalf("invalid JSON output: %v\nstdout: %s", err, stdout)
	}
	if _, ok := env["version"]; !ok {
		t.Error("JSON envelope missing 'version' field")
	}
	if _, ok := env["command"]; !ok {
		t.Error("JSON envelope missing 'command' field")
	}
	if _, ok := env["data"]; !ok {
		t.Error("JSON envelope missing 'data' field")
	}
	return env
}

func TestListJSON(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupInitializedRepo(t)
	rimbaSuccess(t, repo, "add", task1)
	rimbaSuccess(t, repo, "add", task2)

	r := rimbaSuccess(t, repo, "list", "--json")
	env := parseEnvelope(t, r.Stdout)

	if env["command"] != "list" {
		t.Errorf("command = %v, want 'list'", env["command"])
	}

	data, ok := env["data"].([]any)
	if !ok {
		t.Fatalf("data type = %T, want []any", env["data"])
	}
	if len(data) < 2 {
		t.Errorf("expected at least 2 worktrees, got %d", len(data))
	}

	// Verify item structure
	item, ok := data[0].(map[string]any)
	if !ok {
		t.Fatalf("item type = %T, want map[string]any", data[0])
	}
	for _, field := range []string{"task", "type", "branch", "path", "is_current", "status"} {
		if _, exists := item[field]; !exists {
			t.Errorf("item missing field %q", field)
		}
	}

	// No ANSI escape codes
	assertNotContains(t, r.Stdout, "\033[")
	// No spinner output on stderr
	if r.Stderr != "" {
		t.Errorf("expected empty stderr in JSON mode, got: %s", r.Stderr)
	}
}

func TestListJSONFilterNoMatch(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupInitializedRepo(t)
	rimbaSuccess(t, repo, "add", task1)

	// Filter by type that doesn't match any worktree
	r := rimbaSuccess(t, repo, "list", "--type", "bugfix", "--json")
	env := parseEnvelope(t, r.Stdout)

	data, ok := env["data"].([]any)
	if !ok {
		t.Fatalf("data type = %T, want []any", env["data"])
	}
	if len(data) != 0 {
		t.Errorf("expected empty array for non-matching filter, got %d items", len(data))
	}
}

func TestListJSONArchived(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupInitializedRepo(t)
	rimbaSuccess(t, repo, "add", "arch-json")
	rimbaSuccess(t, repo, "remove", "arch-json", "--keep-branch")

	r := rimbaSuccess(t, repo, "list", "--archived", "--json")
	env := parseEnvelope(t, r.Stdout)

	data, ok := env["data"].([]any)
	if !ok {
		t.Fatalf("data type = %T, want []any", env["data"])
	}
	if len(data) == 0 {
		t.Error("expected at least one archived branch")
	}
}

func TestStatusJSON(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupInitializedRepo(t)
	rimbaSuccess(t, repo, "add", "status-json-task")

	r := rimbaSuccess(t, repo, "status", "--json")
	env := parseEnvelope(t, r.Stdout)

	if env["command"] != "status" {
		t.Errorf("command = %v, want 'status'", env["command"])
	}

	data, ok := env["data"].(map[string]any)
	if !ok {
		t.Fatalf("data type = %T, want map[string]any", env["data"])
	}

	// Verify summary
	summary, ok := data["summary"].(map[string]any)
	if !ok {
		t.Fatalf("summary type = %T, want map[string]any", data["summary"])
	}
	if total, _ := summary["total"].(float64); total < 1 {
		t.Errorf("summary.total = %v, want >= 1", total)
	}

	// Verify worktrees array
	worktrees, ok := data["worktrees"].([]any)
	if !ok {
		t.Fatalf("worktrees type = %T, want []any", data["worktrees"])
	}
	if len(worktrees) == 0 {
		t.Error("expected at least one worktree")
	}

	// No ANSI
	assertNotContains(t, r.Stdout, "\033[")
}

func TestConflictCheckJSON(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupCleanInitializedRepo(t)
	conflictSetup(t, repo, taskConflictA, "overlap.txt", "content from a")
	conflictSetup(t, repo, taskConflictB, "overlap.txt", "content from b")

	r := rimbaSuccess(t, repo, "conflict-check", "--json")
	env := parseEnvelope(t, r.Stdout)

	if env["command"] != "conflict-check" {
		t.Errorf("command = %v, want 'conflict-check'", env["command"])
	}

	data, ok := env["data"].(map[string]any)
	if !ok {
		t.Fatalf("data type = %T, want map[string]any", env["data"])
	}

	overlaps, ok := data["overlaps"].([]any)
	if !ok {
		t.Fatalf("overlaps type = %T, want []any", data["overlaps"])
	}
	if len(overlaps) == 0 {
		t.Error("expected at least one overlap")
	}

	// Verify overlap structure
	overlap, ok := overlaps[0].(map[string]any)
	if !ok {
		t.Fatalf("overlap type = %T, want map[string]any", overlaps[0])
	}
	if _, exists := overlap["file"]; !exists {
		t.Error("overlap missing 'file' field")
	}
	if _, exists := overlap["branches"]; !exists {
		t.Error("overlap missing 'branches' field")
	}

	// No ANSI
	assertNotContains(t, r.Stdout, "\033[")
}

func TestConflictCheckJSONNoOverlaps(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupCleanInitializedRepo(t)
	conflictSetup(t, repo, taskConflictA, "a.txt", "content a")
	conflictSetup(t, repo, taskConflictB, "b.txt", "content b")

	r := rimbaSuccess(t, repo, "conflict-check", "--json")
	env := parseEnvelope(t, r.Stdout)

	data, ok := env["data"].(map[string]any)
	if !ok {
		t.Fatalf("data type = %T, want map[string]any", env["data"])
	}
	overlaps, ok := data["overlaps"].([]any)
	if !ok {
		t.Fatalf("overlaps type = %T, want []any", data["overlaps"])
	}
	if len(overlaps) != 0 {
		t.Errorf("expected 0 overlaps, got %d", len(overlaps))
	}
}

func TestExecJSON(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupInitializedRepo(t)
	rimbaSuccess(t, repo, "add", task1)
	rimbaSuccess(t, repo, "add", task2)

	r := rimbaSuccess(t, repo, "exec", "echo hello", "--all", "--json")
	env := parseEnvelope(t, r.Stdout)

	if env["command"] != "exec" {
		t.Errorf("command = %v, want 'exec'", env["command"])
	}

	data, ok := env["data"].(map[string]any)
	if !ok {
		t.Fatalf("data type = %T, want map[string]any", env["data"])
	}

	if data["success"] != true {
		t.Errorf("success = %v, want true", data["success"])
	}
	if data["command"] != "echo hello" {
		t.Errorf("command = %v, want 'echo hello'", data["command"])
	}

	results, ok := data["results"].([]any)
	if !ok {
		t.Fatalf("results type = %T, want []any", data["results"])
	}
	if len(results) < 2 {
		t.Errorf("expected at least 2 results, got %d", len(results))
	}

	// Verify result structure
	result, ok := results[0].(map[string]any)
	if !ok {
		t.Fatalf("result type = %T, want map[string]any", results[0])
	}
	for _, field := range []string{"task", "branch", "path", "exit_code", "stdout", "stderr"} {
		if _, exists := result[field]; !exists {
			t.Errorf("result missing field %q", field)
		}
	}

	// Verify stdout contains "hello"
	found := false
	for _, r := range results {
		item, ok := r.(map[string]any)
		if !ok {
			continue
		}
		stdout, _ := item["stdout"].(string)
		if strings.Contains(stdout, "hello") {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected at least one result with 'hello' in stdout")
	}

	// No ANSI
	assertNotContains(t, r.Stdout, "\033[")
}

func TestExecJSONFailure(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupInitializedRepo(t)
	rimbaSuccess(t, repo, "add", task1)

	r := rimbaFail(t, repo, "exec", "exit 1", "--all", "--json")
	env := parseEnvelope(t, r.Stdout)

	data, ok := env["data"].(map[string]any)
	if !ok {
		t.Fatalf("data type = %T, want map[string]any", env["data"])
	}
	if data["success"] != false {
		t.Errorf("success = %v, want false", data["success"])
	}

	// Stderr should be empty — no text error output in JSON mode
	if r.Stderr != "" {
		t.Errorf("expected empty stderr in JSON failure mode, got: %s", r.Stderr)
	}
}

func TestDepsStatusJSON(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupInitializedRepo(t)
	commitLockfile(t, repo)
	rimbaSuccess(t, repo, "add", "deps-json")

	r := rimbaSuccess(t, repo, "deps", "status", "--json")
	env := parseEnvelope(t, r.Stdout)

	if env["command"] != "deps status" {
		t.Errorf("command = %v, want 'deps status'", env["command"])
	}

	data, ok := env["data"].([]any)
	if !ok {
		t.Fatalf("data type = %T, want []any", env["data"])
	}
	if len(data) < 2 {
		t.Errorf("expected at least 2 worktrees, got %d", len(data))
	}

	// Verify item structure
	item, ok := data[0].(map[string]any)
	if !ok {
		t.Fatalf("item type = %T, want map[string]any", data[0])
	}
	for _, field := range []string{"branch", "path", "modules"} {
		if _, exists := item[field]; !exists {
			t.Errorf("item missing field %q", field)
		}
	}

	// No ANSI
	assertNotContains(t, r.Stdout, "\033[")
}

func TestJSONErrorOutput(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	// Run in a directory that is NOT a git repo to trigger an error
	dir := t.TempDir()
	r := rimbaFail(t, dir, "list", "--json")

	var env map[string]any
	if err := json.Unmarshal([]byte(r.Stdout), &env); err != nil {
		t.Fatalf("invalid JSON error output: %v\nstdout: %s", err, r.Stdout)
	}

	if _, ok := env["error"]; !ok {
		t.Error("error envelope missing 'error' field")
	}
	if _, ok := env["code"]; !ok {
		t.Error("error envelope missing 'code' field")
	}

	// Stderr should be empty — error is in JSON on stdout
	if r.Stderr != "" {
		t.Errorf("expected empty stderr for JSON error, got: %s", r.Stderr)
	}
}

package e2e_test

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/lugassawan/rimba/internal/deps"
	"github.com/lugassawan/rimba/internal/resolver"
	"github.com/lugassawan/rimba/testutil"
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

func TestLogJSON(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupInitializedRepo(t)
	rimbaSuccess(t, repo, "add", "log-json-task")

	// Make a commit in the worktree so log has something to show
	cfg := loadConfig(t, repo)
	wtDir := filepath.Join(repo, cfg.WorktreeDir)
	branch := resolver.BranchName(defaultPrefix, "log-json-task")
	wtPath := resolver.WorktreePath(wtDir, branch)
	testutil.CreateFile(t, wtPath, "work.txt", "some work")
	testutil.GitCmd(t, wtPath, "add", ".")
	testutil.GitCmd(t, wtPath, "commit", "-m", "add log json work")

	r := rimbaSuccess(t, repo, "log", "--json")
	env := parseEnvelope(t, r.Stdout)

	if env["command"] != "log" {
		t.Errorf("command = %v, want 'log'", env["command"])
	}

	data, ok := env["data"].([]any)
	if !ok {
		t.Fatalf("data type = %T, want []any", env["data"])
	}
	if len(data) == 0 {
		t.Fatal("expected at least one log entry")
	}

	var item map[string]any
	for _, d := range data {
		if m, ok := d.(map[string]any); ok && m["task"] == "log-json-task" {
			item = m
			break
		}
	}
	if item == nil {
		t.Fatal("log-json-task entry not found in JSON output")
	}

	for _, field := range []string{"task", "type", "branch", "path", "last_commit", "subject"} {
		if _, exists := item[field]; !exists {
			t.Errorf("item missing field %q", field)
		}
	}
	if item["subject"] != "add log json work" {
		t.Errorf("subject = %v, want 'add log json work'", item["subject"])
	}
	lc, _ := item["last_commit"].(string)
	if _, err := time.Parse(time.RFC3339, lc); err != nil {
		t.Errorf("last_commit %q is not RFC3339: %v", lc, err)
	}

	// No ANSI escape codes
	assertNotContains(t, r.Stdout, "\033[")
	// No spinner output on stderr
	if r.Stderr != "" {
		t.Errorf("expected empty stderr in JSON mode, got: %s", r.Stderr)
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
	commitLockfile(t, repo, deps.LockfilePnpm)
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

func TestAddJSONE2E(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupInitializedRepo(t)

	r := rimbaSuccess(t, repo, "add", "add-json-task", "--json")
	env := parseEnvelope(t, r.Stdout)

	data, ok := env["data"].(map[string]any)
	if !ok {
		t.Fatalf("data type = %T, want map[string]any", env["data"])
	}
	if data["mode"] != "task" {
		t.Errorf("mode = %v, want 'task'", data["mode"])
	}
	if branch, _ := data["branch"].(string); branch == "" {
		t.Error("expected non-empty branch")
	}

	assertNotContains(t, r.Stdout, "\033[")
	if r.Stderr != "" {
		t.Errorf("expected empty stderr in JSON mode, got: %s", r.Stderr)
	}
}

func TestMergeJSONE2E(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupCleanInitializedRepo(t)
	mergeSetup(t, repo, "merge-json-task")

	r := rimbaSuccess(t, repo, "merge", "merge-json-task", "--json")
	env := parseEnvelope(t, r.Stdout)

	data, ok := env["data"].(map[string]any)
	if !ok {
		t.Fatalf("data type = %T, want map[string]any", env["data"])
	}
	if data["source_removed"] != true {
		t.Errorf("source_removed = %v, want true", data["source_removed"])
	}

	assertNotContains(t, r.Stdout, "\033[")
	if r.Stderr != "" {
		t.Errorf("expected empty stderr in JSON mode, got: %s", r.Stderr)
	}
}

func TestRemoveJSONE2E(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupInitializedRepo(t)
	rimbaSuccess(t, repo, "add", "remove-json-task")

	r := rimbaSuccess(t, repo, "remove", "remove-json-task", "--json")
	env := parseEnvelope(t, r.Stdout)

	data, ok := env["data"].(map[string]any)
	if !ok {
		t.Fatalf("data type = %T, want map[string]any", env["data"])
	}
	if data["worktree_removed"] != true {
		t.Errorf("worktree_removed = %v, want true", data["worktree_removed"])
	}

	assertNotContains(t, r.Stdout, "\033[")
	if r.Stderr != "" {
		t.Errorf("expected empty stderr in JSON mode, got: %s", r.Stderr)
	}
}

func TestRenameJSONE2E(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupInitializedRepo(t)
	rimbaSuccess(t, repo, "add", "rename-json-old")

	r := rimbaSuccess(t, repo, "rename", "rename-json-old", "rename-json-new", "--json")
	env := parseEnvelope(t, r.Stdout)

	data, ok := env["data"].(map[string]any)
	if !ok {
		t.Fatalf("data type = %T, want map[string]any", env["data"])
	}
	newBranch, _ := data["new_branch"].(string)
	if !strings.Contains(newBranch, "rename-json-new") {
		t.Errorf("new_branch = %v, want to contain 'rename-json-new'", newBranch)
	}

	assertNotContains(t, r.Stdout, "\033[")
	if r.Stderr != "" {
		t.Errorf("expected empty stderr in JSON mode, got: %s", r.Stderr)
	}
}

func TestCleanMergedJSONE2E(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupCleanInitializedRepo(t)
	cleanMergeSetup(t, repo, "clean-json-task")

	r := rimbaSuccess(t, repo, "clean", flagMergedE2E, flagForceE2E, "--json")
	env := parseEnvelope(t, r.Stdout)

	data, ok := env["data"].(map[string]any)
	if !ok {
		t.Fatalf("data type = %T, want map[string]any", env["data"])
	}
	if data["mode"] != "merged" {
		t.Errorf("mode = %v, want 'merged'", data["mode"])
	}
	if cleanedCount, _ := data["cleaned_count"].(float64); cleanedCount != 1 {
		t.Errorf("cleaned_count = %v, want 1", data["cleaned_count"])
	}

	assertNotContains(t, r.Stdout, "\033[")
	if r.Stderr != "" {
		t.Errorf("expected empty stderr in JSON mode, got: %s", r.Stderr)
	}
}

func TestCleanMergedJSONNoForceE2E(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupCleanInitializedRepo(t)
	cleanMergeSetup(t, repo, "clean-json-noforce")

	r := rimbaFail(t, repo, "clean", flagMergedE2E, "--json")

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

	assertNotContains(t, r.Stdout, "\033[")
	if r.Stderr != "" {
		t.Errorf("expected empty stderr for JSON error, got: %s", r.Stderr)
	}
}

func TestSyncJSONE2E(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupCleanInitializedRepo(t)
	rimbaSuccess(t, repo, "add", "sync-json-task")
	commitOnMain(t, repo)

	r := rimbaSuccess(t, repo, "sync", "sync-json-task", "--json")
	env := parseEnvelope(t, r.Stdout)

	data, ok := env["data"].(map[string]any)
	if !ok {
		t.Fatalf("data type = %T, want map[string]any", env["data"])
	}
	summary, ok := data["summary"].(map[string]any)
	if !ok {
		t.Fatalf("summary type = %T, want map[string]any", data["summary"])
	}
	if summary["synced"] != float64(1) {
		t.Errorf("summary.synced = %v, want 1", summary["synced"])
	}

	assertNotContains(t, r.Stdout, "\033[")
	if r.Stderr != "" {
		t.Errorf("expected empty stderr in JSON mode, got: %s", r.Stderr)
	}
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

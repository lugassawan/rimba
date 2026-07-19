package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lugassawan/rimba/internal/config"
	"github.com/lugassawan/rimba/internal/observability"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// recordingHandler returns a server.ToolHandlerFunc that records whether a
// Recorder was attached to ctx, then returns result.
func recordingHandler(sawRecorder *bool, result *mcp.CallToolResult) server.ToolHandlerFunc {
	return func(ctx context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		*sawRecorder = observability.FromContext(ctx) != nil
		return result, nil
	}
}

func TestWithRecorderNilConfigSkipsWrapping(t *testing.T) {
	hctx := &HandlerContext{Config: nil, RepoRoot: "/repo", Version: "test"}
	var sawRecorder bool
	handler := withRecorder(hctx, "add", recordingHandler(&sawRecorder, mcp.NewToolResultText("ok")))

	result, err := handler(context.Background(), mcp.CallToolRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sawRecorder {
		t.Error("expected no Recorder attached when Config is nil")
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
}

func TestWithRecorderDisabledConfigSkipsWrapping(t *testing.T) {
	disabled := false
	hctx := &HandlerContext{
		Config:   &config.Config{Observability: &config.ObservabilityConfig{Enabled: &disabled}},
		RepoRoot: "/repo",
		Version:  "test",
	}
	var sawRecorder bool
	handler := withRecorder(hctx, "add", recordingHandler(&sawRecorder, mcp.NewToolResultText("ok")))

	if _, err := handler(context.Background(), mcp.CallToolRequest{}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sawRecorder {
		t.Error("expected no Recorder attached when observability is disabled")
	}
}

// withRedirectedCacheDir points os.UserCacheDir() at a fresh temp dir for the
// duration of the test, so enabled-observability tests never touch the real
// user cache dir (mirrors sink_test.go's HOME-override pattern).
func withRedirectedCacheDir(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	// Linux resolves UserCacheDir via XDG_CACHE_HOME when set; clear it so
	// HOME/.cache takes effect consistently across platforms.
	t.Setenv("XDG_CACHE_HOME", "")
	os.Unsetenv("XDG_CACHE_HOME")
	return home
}

func TestWithRecorderEnabledAttachesRecorderAndWritesFile(t *testing.T) {
	home := withRedirectedCacheDir(t)
	repoRoot := t.TempDir()

	hctx := &HandlerContext{
		Config:   &config.Config{},
		RepoRoot: repoRoot,
		Version:  "test",
	}
	var sawRecorder bool
	handler := withRecorder(hctx, "add", recordingHandler(&sawRecorder, mcp.NewToolResultText("ok")))

	if _, err := handler(context.Background(), mcp.CallToolRequest{}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !sawRecorder {
		t.Fatal("expected a Recorder attached to ctx when observability is enabled")
	}

	logFiles := findCacheLogFiles(t, home)
	if len(logFiles) == 0 {
		t.Fatal("expected at least one .log.jsonl file to be written under the cache dir")
	}

	data, err := os.ReadFile(logFiles[0])
	if err != nil {
		t.Fatalf("reading log file: %v", err)
	}
	if !containsCommandRecord(t, data, "add") {
		t.Errorf("log file %s did not contain a CommandRecord for command %q:\n%s", logFiles[0], "add", data)
	}
}

func TestWithRecorderErrorResultMarksOutcomeError(t *testing.T) {
	home := withRedirectedCacheDir(t)
	repoRoot := t.TempDir()

	hctx := &HandlerContext{
		Config:   &config.Config{},
		RepoRoot: repoRoot,
		Version:  "test",
	}
	var sawRecorder bool
	handler := withRecorder(hctx, "remove", recordingHandler(&sawRecorder, errorResult(errors.New("boom"))))

	if _, err := handler(context.Background(), mcp.CallToolRequest{}); err != nil {
		t.Fatalf("unexpected protocol error: %v", err)
	}

	logFiles := findCacheLogFiles(t, home)
	if len(logFiles) == 0 {
		t.Fatal("expected at least one .log.jsonl file to be written under the cache dir")
	}
	data, err := os.ReadFile(logFiles[0])
	if err != nil {
		t.Fatalf("reading log file: %v", err)
	}
	if !containsOutcome(t, data, observability.OutcomeError) {
		t.Errorf("log file did not contain an error outcome CommandRecord:\n%s", data)
	}
}

// findCacheLogFiles walks home looking for any *.log.jsonl file, wherever
// os.UserCacheDir() placed the "rimba" subdir on this platform (e.g.
// Library/Caches/rimba on macOS, .cache/rimba on Linux).
func findCacheLogFiles(t *testing.T, home string) []string {
	t.Helper()
	var matches []string
	err := filepath.WalkDir(home, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		if strings.HasSuffix(path, ".log.jsonl") && filepath.Base(filepath.Dir(path)) == "rimba" {
			matches = append(matches, path)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walking %s: %v", home, err)
	}
	return matches
}

// containsCommandRecord reports whether data (JSONL) contains a "command" kind
// record for the given command name.
func containsCommandRecord(t *testing.T, data []byte, command string) bool {
	t.Helper()
	return scanJSONLKind(t, data, "command", func(m map[string]any) bool {
		return m["command"] == command
	})
}

// containsOutcome reports whether data (JSONL) contains a "command" kind
// record with the given outcome.
func containsOutcome(t *testing.T, data []byte, outcome string) bool {
	t.Helper()
	return scanJSONLKind(t, data, "command", func(m map[string]any) bool {
		return m["outcome"] == outcome
	})
}

// scanJSONLKind scans newline-delimited JSON records, returning true if any
// record has the given "kind" and satisfies match.
func scanJSONLKind(t *testing.T, data []byte, kind string, match func(map[string]any) bool) bool {
	t.Helper()
	for line := range bytes.SplitSeq(data, []byte("\n")) {
		if len(line) == 0 {
			continue
		}
		var m map[string]any
		if err := json.Unmarshal(line, &m); err != nil {
			t.Fatalf("unmarshal jsonl line: %v", err)
		}
		if m["kind"] == kind && match(m) {
			return true
		}
	}
	return false
}

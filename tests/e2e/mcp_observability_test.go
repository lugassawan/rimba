package e2e_test

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// mcpObservabilityEnv returns the environment for an MCP session that records
// observability into home. RIMBA_NO_OBSERVABILITY disables on mere presence,
// so it is dropped rather than overridden.
func mcpObservabilityEnv(home string) []string {
	var env []string
	for _, kv := range os.Environ() {
		if !strings.HasPrefix(kv, "RIMBA_NO_OBSERVABILITY=") && !strings.HasPrefix(kv, "XDG_CACHE_HOME=") {
			env = append(env, kv)
		}
	}
	return append(env, "GOCOVERDIR="+coverDir, "NO_COLOR=1", "HOME="+home)
}

// findDayFiles returns the log and metrics day-files found anywhere under home.
func findDayFiles(t *testing.T, home string) (logs, metrics []string) {
	t.Helper()
	err := filepath.WalkDir(home, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil || d.IsDir() {
			return walkErr
		}
		switch {
		case strings.HasSuffix(path, ".log.jsonl"):
			logs = append(logs, path)
		case strings.HasSuffix(path, ".metrics.jsonl"):
			metrics = append(metrics, path)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walking %s: %v", home, err)
	}
	return logs, metrics
}

// assertOccurrences fails unless needle appears want times in the file at path.
func assertOccurrences(t *testing.T, path, needle string, want int) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	if got := strings.Count(string(data), needle); got != want {
		t.Errorf("%s occurrences of %s = %d, want %d:\n%s", filepath.Base(path), needle, got, want, data)
	}
}

// TestMCPSharedSinkRecordsEveryCall runs one `rimba mcp` session with
// observability on and checks that a single shared sink captured both calls.
func TestMCPSharedSinkRecordsEveryCall(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupInitializedRepo(t)
	home := t.TempDir()

	var stdin bytes.Buffer
	for _, msg := range []string{
		mcpInitRequest(),
		mcpNotification(),
		jsonRPCRequest(2, "tools/call", map[string]any{"name": "list", "arguments": map[string]any{}}),
		jsonRPCRequest(3, "tools/call", map[string]any{"name": "list", "arguments": map[string]any{}}),
	} {
		stdin.WriteString(msg)
	}

	cmd := exec.Command(binaryPath, "mcp")
	cmd.Dir = repo
	cmd.Stdin = &stdin
	cmd.Env = mcpObservabilityEnv(home)
	_ = cmd.Run() // non-zero exit on stdin close is fine

	logs, metrics := findDayFiles(t, home)
	if len(logs) != 1 || len(metrics) != 1 {
		t.Fatalf("day files: logs=%v metrics=%v, want exactly one of each", logs, metrics)
	}
	assertOccurrences(t, logs[0], `"kind":"command"`, 2)
	assertOccurrences(t, metrics[0], `"name":"command"`, 2)
}

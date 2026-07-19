package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lugassawan/rimba/internal/observability"
	"github.com/lugassawan/rimba/internal/output"
)

// testRepoRoot is a fake repoRoot shared by tests that only need a stable
// path to derive an observability.RepoPrefix from.
const testRepoRoot = "/repo/myproject"

// writeReportFile writes lines (already-formed JSON strings, one per line)
// to cacheDir/rimba/name, creating the directory as needed.
func writeReportFile(t *testing.T, cacheDir, name string, lines []string) {
	t.Helper()
	dir := filepath.Join(cacheDir, "rimba")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	content := strings.Join(lines, "\n") + "\n"
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil { //nolint:gosec
		t.Fatalf("WriteFile: %v", err)
	}
}

// marshalSpan is a small helper to keep fixture lines terse in tests.
func marshalSpan(t *testing.T, s observability.SpanRecord) string {
	t.Helper()
	s.SchemaVersion = observability.SchemaVersion
	s.Kind = "span"
	b, err := json.Marshal(s)
	if err != nil {
		t.Fatalf("json.Marshal(SpanRecord): %v", err)
	}
	return string(b)
}

func marshalCommand(t *testing.T, c observability.CommandRecord) string {
	t.Helper()
	c.SchemaVersion = observability.SchemaVersion
	c.Kind = "command"
	b, err := json.Marshal(c)
	if err != nil {
		t.Fatalf("json.Marshal(CommandRecord): %v", err)
	}
	return string(b)
}

func TestCollectReportDataNoFiles(t *testing.T) {
	cacheDir := t.TempDir()
	data := collectReportData(cacheDir, "/repo/myproject")

	if len(data.Phases) != 0 {
		t.Errorf("Phases = %v, want empty", data.Phases)
	}
	if data.Phases == nil {
		t.Error("Phases is nil, want non-nil empty slice")
	}
	if data.Env.UnparseableLines != 0 {
		t.Errorf("UnparseableLines = %d, want 0", data.Env.UnparseableLines)
	}
}

func TestCollectReportDataAggregatesSpans(t *testing.T) {
	cacheDir := t.TempDir()
	repoRoot := testRepoRoot
	prefix := observability.RepoPrefix(repoRoot)

	lines := []string{
		marshalSpan(t, observability.SpanRecord{Command: wantAddCommand, Name: "clone", DurationMS: 100}),
		marshalSpan(t, observability.SpanRecord{Command: wantAddCommand, Name: "clone", DurationMS: 200}),
		marshalSpan(t, observability.SpanRecord{Command: wantAddCommand, Name: "clone", DurationMS: 300}),
		marshalSpan(t, observability.SpanRecord{Command: wantAddCommand, Name: "clone", DurationMS: 400}),
		marshalSpan(t, observability.SpanRecord{Command: wantAddCommand, Name: "command", DurationMS: 500}),
		marshalSpan(t, observability.SpanRecord{Command: "sync", Name: "command", DurationMS: 50}),
	}
	writeReportFile(t, cacheDir, prefix+"-2026-07-19.metrics.jsonl", lines)

	data := collectReportData(cacheDir, repoRoot)

	if data.Env.UnparseableLines != 0 {
		t.Errorf("UnparseableLines = %d, want 0", data.Env.UnparseableLines)
	}
	if len(data.Phases) != 3 {
		t.Fatalf("len(Phases) = %d, want 3: %+v", len(data.Phases), data.Phases)
	}

	// Sorted by Command then Phase: "add"/"clone", "add"/"command", "sync"/"command".
	clone := data.Phases[0]
	if clone.Command != wantAddCommand || clone.Phase != "clone" {
		t.Fatalf("Phases[0] = %+v, want add/clone", clone)
	}
	if clone.Count != 4 {
		t.Errorf("Count = %d, want 4", clone.Count)
	}
	// durations [100,200,300,400] sorted; p50 idx=int(4*0.5)=2 -> 300; p95 idx=int(4*0.95)=3 -> 400.
	if clone.P50MS != 300 {
		t.Errorf("P50MS = %v, want 300", clone.P50MS)
	}
	if clone.P95MS != 400 {
		t.Errorf("P95MS = %v, want 400", clone.P95MS)
	}
	if clone.MeanMS != 250 {
		t.Errorf("MeanMS = %v, want 250", clone.MeanMS)
	}
}

func TestCollectReportDataUnparseableLines(t *testing.T) {
	cacheDir := t.TempDir()
	repoRoot := testRepoRoot
	prefix := observability.RepoPrefix(repoRoot)

	lines := []string{
		marshalSpan(t, observability.SpanRecord{Command: wantAddCommand, Name: "command", DurationMS: 10}),
		"{not valid json",
		`{"schema_version":999,"kind":"span","command":"add","name":"command","duration_ms":20}`,
	}
	writeReportFile(t, cacheDir, prefix+"-2026-07-19.metrics.jsonl", lines)

	data := collectReportData(cacheDir, repoRoot)

	if data.Env.UnparseableLines != 2 {
		t.Errorf("UnparseableLines = %d, want 2", data.Env.UnparseableLines)
	}
	if len(data.Phases) != 1 {
		t.Fatalf("len(Phases) = %d, want 1: %+v", len(data.Phases), data.Phases)
	}
	if data.Phases[0].Count != 1 {
		t.Errorf("Count = %d, want 1 (garbage/bad-schema lines must not appear)", data.Phases[0].Count)
	}
}

func TestCollectReportDataCollectsVersionsFromCommandRecords(t *testing.T) {
	cacheDir := t.TempDir()
	repoRoot := testRepoRoot
	prefix := observability.RepoPrefix(repoRoot)

	writeReportFile(t, cacheDir, prefix+"-2026-07-18.log.jsonl", []string{
		marshalCommand(t, observability.CommandRecord{Command: wantAddCommand, RimbaVersion: "1.2.0"}),
		marshalCommand(t, observability.CommandRecord{Command: "sync", RimbaVersion: "1.3.0"}),
	})
	writeReportFile(t, cacheDir, prefix+"-2026-07-19.log.jsonl", []string{
		marshalCommand(t, observability.CommandRecord{Command: "status", RimbaVersion: "1.3.0"}),
	})

	data := collectReportData(cacheDir, repoRoot)

	want := []string{"1.2.0", "1.3.0"}
	if len(data.Env.RimbaVersionsSeen) != len(want) {
		t.Fatalf("RimbaVersionsSeen = %v, want %v", data.Env.RimbaVersionsSeen, want)
	}
	for i, v := range want {
		if data.Env.RimbaVersionsSeen[i] != v {
			t.Errorf("RimbaVersionsSeen[%d] = %q, want %q", i, data.Env.RimbaVersionsSeen[i], v)
		}
	}
}

func TestCollectReportDataIsolatesByRepo(t *testing.T) {
	cacheDir := t.TempDir()
	repoA := "/repo/alpha"
	repoB := "/repo/beta"

	writeReportFile(t, cacheDir, observability.RepoPrefix(repoA)+"-2026-07-19.metrics.jsonl", []string{
		marshalSpan(t, observability.SpanRecord{Command: "alpha-cmd", Name: "command", DurationMS: 111}),
	})
	writeReportFile(t, cacheDir, observability.RepoPrefix(repoB)+"-2026-07-19.metrics.jsonl", []string{
		marshalSpan(t, observability.SpanRecord{Command: "beta-cmd", Name: "command", DurationMS: 222}),
	})

	data := collectReportData(cacheDir, repoA)

	if len(data.Phases) != 1 {
		t.Fatalf("len(Phases) = %d, want 1: %+v", len(data.Phases), data.Phases)
	}
	if data.Phases[0].Command != "alpha-cmd" {
		t.Errorf("Command = %q, want alpha-cmd (repo B's data leaked in)", data.Phases[0].Command)
	}
}

func TestReportCmdNoDataTextMode(t *testing.T) {
	restore := overrideNewRunner(&mockRunner{
		run: func(args ...string) (string, error) {
			if len(args) >= 2 && args[1] == cmdGitCommonDir {
				return filepath.Join(repoPath, ".git"), nil
			}
			return "", nil
		},
		runInDir: noopRunInDir,
	})
	defer restore()

	cacheDir := t.TempDir()
	t.Setenv("HOME", cacheDir)
	t.Setenv("XDG_CACHE_HOME", "")

	cmd, buf := newTestCmd()

	if err := reportCmd.RunE(cmd, nil); err != nil {
		t.Fatalf("reportCmd.RunE: %v", err)
	}

	if !strings.Contains(buf.String(), "No observability data found") {
		t.Errorf("expected 'No observability data found', got: %q", buf.String())
	}
}

func TestReportCmdTextModeWithData(t *testing.T) {
	restore := overrideNewRunner(&mockRunner{
		run: func(args ...string) (string, error) {
			if len(args) >= 2 && args[1] == cmdGitCommonDir {
				return filepath.Join(repoPath, ".git"), nil
			}
			return "", nil
		},
		runInDir: noopRunInDir,
	})
	defer restore()

	cacheDir := t.TempDir()
	t.Setenv("HOME", cacheDir)
	t.Setenv("XDG_CACHE_HOME", "")

	prefix := observability.RepoPrefix(repoPath)
	// os.UserCacheDir() resolves relative to $HOME; write to both possible
	// per-OS locations so this test is platform-agnostic (see
	// TestReportCmdJSONRoundTrip for the same pattern).
	for _, sub := range []string{"Library/Caches", ".cache"} {
		writeReportFile(t, filepath.Join(cacheDir, sub), prefix+"-2026-07-19.metrics.jsonl", []string{
			marshalSpan(t, observability.SpanRecord{Command: wantAddCommand, Name: "command", DurationMS: 42}),
		})
	}

	cmd, buf := newTestCmd()
	_ = cmd.Flags().Set(flagNoColor, "true")

	if err := reportCmd.RunE(cmd, nil); err != nil {
		t.Fatalf("reportCmd.RunE: %v", err)
	}

	out := buf.String()
	for _, want := range []string{"COMMAND", "PHASE", "COUNT", "P50", "P95", "MEAN", wantAddCommand, "command", "42.0ms"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q, got: %q", want, out)
		}
	}
	if strings.Contains(out, "No observability data found") {
		t.Errorf("expected populated table, got 'no data' message: %q", out)
	}
}

func TestReportCmdJSONRoundTrip(t *testing.T) {
	restore := overrideNewRunner(&mockRunner{
		run: func(args ...string) (string, error) {
			if len(args) >= 2 && args[1] == cmdGitCommonDir {
				return filepath.Join(repoPath, ".git"), nil
			}
			return "", nil
		},
		runInDir: noopRunInDir,
	})
	defer restore()

	cacheDir := t.TempDir()
	t.Setenv("HOME", cacheDir)
	t.Setenv("XDG_CACHE_HOME", "")

	prefix := observability.RepoPrefix(repoPath)
	// os.UserCacheDir() resolves relative to $HOME; on darwin that's
	// $HOME/Library/Caches, on linux $HOME/.cache. Write to both possible
	// locations so this test is platform-agnostic; only one will exist per OS.
	for _, sub := range []string{"Library/Caches", ".cache"} {
		writeReportFile(t, filepath.Join(cacheDir, sub), prefix+"-2026-07-19.metrics.jsonl", []string{
			marshalSpan(t, observability.SpanRecord{Command: wantAddCommand, Name: "command", DurationMS: 42}),
		})
	}

	cmd, buf := newTestCmd()
	_ = cmd.Flags().Set(flagJSON, "true")

	if err := reportCmd.RunE(cmd, nil); err != nil {
		t.Fatalf("reportCmd.RunE: %v", err)
	}

	var envelope struct {
		Data output.ReportData `json:"data"`
	}
	if err := json.Unmarshal(buf.Bytes(), &envelope); err != nil {
		t.Fatalf("json.Unmarshal: %v\noutput: %s", err, buf.String())
	}

	if envelope.Data.Env.OS == "" {
		t.Error("Env.OS is empty")
	}
	if len(envelope.Data.Phases) != 1 {
		t.Fatalf("len(Phases) = %d, want 1: %+v", len(envelope.Data.Phases), envelope.Data.Phases)
	}
	if envelope.Data.Phases[0].Command != wantAddCommand {
		t.Errorf("Command = %q, want %q", envelope.Data.Phases[0].Command, wantAddCommand)
	}
}

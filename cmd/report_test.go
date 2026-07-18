package cmd

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lugassawan/rimba/internal/config"
	"github.com/lugassawan/rimba/internal/metrics"
	"github.com/lugassawan/rimba/internal/output"
	"github.com/spf13/cobra"
)

// reportFixtureLine builds one JSONL metrics.Run record with a single span.
func reportFixtureLine(t *testing.T, command, timestamp, spanName string, spanDurationMS int64) string {
	t.Helper()
	run := metrics.Run{
		SchemaVersion: metrics.SchemaVersion,
		Timestamp:     timestamp,
		Command:       command,
		Machine:       metrics.MachineInfo{OS: "linux", Arch: "amd64", NumCPU: 4},
		Spans:         []metrics.Span{{Name: spanName, DurationMS: spanDurationMS}},
	}
	data, err := json.Marshal(run)
	if err != nil {
		t.Fatalf("marshal fixture run: %v", err)
	}
	return string(data)
}

// writeMetricsFixture writes lines (already-serialized JSONL) to
// <root>/.rimba/metrics.jsonl.
func writeMetricsFixture(t *testing.T, root string, lines []string) {
	t.Helper()
	dir := filepath.Join(root, config.DirName)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	path := filepath.Join(dir, metricsFileName)
	content := strings.Join(lines, "\n") + "\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
}

// newReportTestCmd builds a report-ready test command (newTestCmd plus
// report's own --last/--command flags) rooted at a fresh repo whose
// MainRepoRoot resolves to root.
func newReportTestCmd(t *testing.T, root string) (*cobra.Command, *bytes.Buffer) {
	t.Helper()
	restore := overrideNewRunner(repoRootRunner(root, nil))
	t.Cleanup(restore)

	cmd, buf := newTestCmd()
	cmd.Flags().Int(flagLast, 0, "")
	cmd.Flags().String(flagCommand, "", "")
	return cmd, buf
}

func setJSON(t *testing.T, cmd *cobra.Command) {
	t.Helper()
	if err := cmd.Flags().Set(flagJSON, "true"); err != nil {
		t.Fatalf("set --json: %v", err)
	}
}

func TestReportNoMetricsFileTable(t *testing.T) {
	root := t.TempDir()
	cmd, buf := newReportTestCmd(t, root)

	if err := reportCmd.RunE(cmd, nil); err != nil {
		t.Fatalf("reportCmd.RunE: %v", err)
	}
	if !strings.Contains(buf.String(), msgNoMetrics) {
		t.Errorf("output = %q, want it to contain %q", buf.String(), msgNoMetrics)
	}
}

func TestReportNoMetricsFileJSON(t *testing.T) {
	root := t.TempDir()
	cmd, buf := newReportTestCmd(t, root)
	setJSON(t, cmd)

	if err := reportCmd.RunE(cmd, nil); err != nil {
		t.Fatalf("reportCmd.RunE: %v", err)
	}

	var env output.Envelope
	if err := json.Unmarshal(buf.Bytes(), &env); err != nil {
		t.Fatalf("unmarshal envelope: %v\noutput: %s", err, buf.String())
	}
	if env.Command != "report" {
		t.Errorf("env.Command = %q, want %q", env.Command, "report")
	}

	data, err := json.Marshal(env.Data)
	if err != nil {
		t.Fatalf("re-marshal data: %v", err)
	}
	var reportData output.ReportData
	if err := json.Unmarshal(data, &reportData); err != nil {
		t.Fatalf("unmarshal ReportData: %v", err)
	}
	if reportData.Commands == nil {
		t.Fatal("Commands = nil, want empty non-nil slice")
	}
	if len(reportData.Commands) != 0 {
		t.Errorf("len(Commands) = %d, want 0", len(reportData.Commands))
	}
}

// TestReportEmptyMetricsFileBehavesLikeMissing covers the present-but-empty
// file edge case alongside the fully-missing-file case above.
func TestReportEmptyMetricsFileBehavesLikeMissing(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, config.DirName)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, metricsFileName), []byte{}, 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	cmd, buf := newReportTestCmd(t, root)
	if err := reportCmd.RunE(cmd, nil); err != nil {
		t.Fatalf("reportCmd.RunE: %v", err)
	}
	if !strings.Contains(buf.String(), msgNoMetrics) {
		t.Errorf("output = %q, want it to contain %q", buf.String(), msgNoMetrics)
	}
}

func TestReportTableRendersCommandsAndPhases(t *testing.T) {
	root := t.TempDir()
	writeMetricsFixture(t, root, []string{
		reportFixtureLine(t, wantAddCommand, "2026-01-01T00:00:00Z", "copy", 10),
		reportFixtureLine(t, wantAddCommand, "2026-01-02T00:00:00Z", "copy", 20),
	})

	cmd, buf := newReportTestCmd(t, root)
	if err := reportCmd.RunE(cmd, nil); err != nil {
		t.Fatalf("reportCmd.RunE: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "add (2 run(s))") {
		t.Errorf("output missing command header, got: %q", out)
	}
	if !strings.Contains(out, "copy") || !strings.Contains(out, "count=2") {
		t.Errorf("output missing phase row, got: %q", out)
	}
}

func TestReportJSONRendersCommandsAndPhases(t *testing.T) {
	root := t.TempDir()
	writeMetricsFixture(t, root, []string{
		reportFixtureLine(t, wantAddCommand, "2026-01-01T00:00:00Z", "copy", 10),
		reportFixtureLine(t, wantAddCommand, "2026-01-02T00:00:00Z", "copy", 20),
	})

	cmd, buf := newReportTestCmd(t, root)
	setJSON(t, cmd)
	if err := reportCmd.RunE(cmd, nil); err != nil {
		t.Fatalf("reportCmd.RunE: %v", err)
	}

	var env output.Envelope
	if err := json.Unmarshal(buf.Bytes(), &env); err != nil {
		t.Fatalf("unmarshal envelope: %v", err)
	}
	data, err := json.Marshal(env.Data)
	if err != nil {
		t.Fatalf("re-marshal data: %v", err)
	}
	var reportData output.ReportData
	if err := json.Unmarshal(data, &reportData); err != nil {
		t.Fatalf("unmarshal ReportData: %v", err)
	}

	if len(reportData.Commands) != 1 {
		t.Fatalf("len(Commands) = %d, want 1", len(reportData.Commands))
	}
	got := reportData.Commands[0]
	if got.Command != wantAddCommand || got.Count != 2 {
		t.Errorf("Commands[0] = %+v, want Command=%s Count=2", got, wantAddCommand)
	}
	if len(got.Phases) != 1 || got.Phases[0].Name != "copy" || got.Phases[0].Count != 2 {
		t.Errorf("Phases = %+v, want one 'copy' phase with count 2", got.Phases)
	}
	if got.Phases[0].P50MS != 10 || got.Phases[0].MeanMS != 15 {
		t.Errorf("Phases[0] = %+v, want P50MS=10 MeanMS=15", got.Phases[0])
	}
}

func TestReportCommandFilter(t *testing.T) {
	root := t.TempDir()
	writeMetricsFixture(t, root, []string{
		reportFixtureLine(t, wantAddCommand, "2026-01-01T00:00:00Z", "copy", 10),
		reportFixtureLine(t, "rename", "2026-01-02T00:00:00Z", "deps", 30),
	})

	cmd, buf := newReportTestCmd(t, root)
	if err := cmd.Flags().Set(flagCommand, "rename"); err != nil {
		t.Fatalf("set --command: %v", err)
	}
	if err := reportCmd.RunE(cmd, nil); err != nil {
		t.Fatalf("reportCmd.RunE: %v", err)
	}

	out := buf.String()
	if strings.Contains(out, "add (") {
		t.Errorf("expected 'add' to be filtered out, got: %q", out)
	}
	if !strings.Contains(out, "rename (1 run(s))") {
		t.Errorf("expected 'rename' report, got: %q", out)
	}
}

func TestReportLastFilterKeepsMostRecentOfFilteredSet(t *testing.T) {
	root := t.TempDir()
	writeMetricsFixture(t, root, []string{
		reportFixtureLine(t, wantAddCommand, "2026-01-01T00:00:00Z", "copy", 10),
		reportFixtureLine(t, "rename", "2026-01-02T00:00:00Z", "deps", 999),
		reportFixtureLine(t, wantAddCommand, "2026-01-03T00:00:00Z", "copy", 50),
	})

	cmd, buf := newReportTestCmd(t, root)
	// --command first narrows to the two "add" runs (10ms, 50ms), THEN
	// --last 1 keeps only the most recent of that filtered set (50ms) —
	// not the 999ms "rename" run that --last alone (applied first) would
	// have kept.
	if err := cmd.Flags().Set(flagCommand, wantAddCommand); err != nil {
		t.Fatalf("set --command: %v", err)
	}
	if err := cmd.Flags().Set(flagLast, "1"); err != nil {
		t.Fatalf("set --last: %v", err)
	}
	if err := reportCmd.RunE(cmd, nil); err != nil {
		t.Fatalf("reportCmd.RunE: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "add (1 run(s))") {
		t.Errorf("expected exactly 1 filtered 'add' run, got: %q", out)
	}
	if !strings.Contains(out, "p50=50") {
		t.Errorf("expected the most recent (50ms) run to survive filtering, got: %q", out)
	}
}

func TestFilterRunsCommandThenLastOrdering(t *testing.T) {
	runs := []metrics.Run{
		{Command: wantAddCommand, DurationMS: 1},
		{Command: "rename", DurationMS: 2},
		{Command: wantAddCommand, DurationMS: 3},
		{Command: wantAddCommand, DurationMS: 4},
	}

	got := filterRuns(runs, wantAddCommand, 2)

	if len(got) != 2 {
		t.Fatalf("len(got) = %d, want 2", len(got))
	}
	if got[0].DurationMS != 3 || got[1].DurationMS != 4 {
		t.Errorf("got = %+v, want the last 2 of the 'add'-filtered set (3, 4)", got)
	}
}

func TestFilterRunsNoFiltersReturnsAll(t *testing.T) {
	runs := []metrics.Run{{Command: wantAddCommand}, {Command: "rename"}}

	got := filterRuns(runs, "", 0)
	if len(got) != 2 {
		t.Fatalf("len(got) = %d, want 2 (no filters applied)", len(got))
	}
}

func TestFilterRunsLastExceedsLengthReturnsAll(t *testing.T) {
	runs := []metrics.Run{{Command: wantAddCommand}, {Command: wantAddCommand}}

	got := filterRuns(runs, "", 10)
	if len(got) != 2 {
		t.Fatalf("len(got) = %d, want 2 (last exceeds available count)", len(got))
	}
}

func TestReportReadRunsErrorPropagates(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, config.DirName)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	// A malformed line makes metrics.ReadRuns return an error, which
	// reportCmd.RunE must propagate rather than swallow.
	if err := os.WriteFile(filepath.Join(dir, metricsFileName), []byte("not json\n"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	cmd, _ := newReportTestCmd(t, root)
	if err := reportCmd.RunE(cmd, nil); err == nil {
		t.Fatal("expected error from malformed metrics.jsonl")
	}
}

func TestReportMainRepoRootErrorPropagates(t *testing.T) {
	restore := overrideNewRunner(notGitRepoRunner())
	defer restore()

	cmd, _ := newTestCmd()
	cmd.Flags().Int(flagLast, 0, "")
	cmd.Flags().String(flagCommand, "", "")

	if err := reportCmd.RunE(cmd, nil); err == nil {
		t.Fatal("expected error when not in a git repository")
	}
}

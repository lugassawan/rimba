package metrics

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// cmdAdd is the fixture command name shared across several report tests
// below (goconst wants ≥3 identical literals pulled into a constant).
const cmdAdd = "add"

func TestReadRunsMissingFileReturnsNilNil(t *testing.T) {
	path := filepath.Join(t.TempDir(), "does-not-exist.jsonl")

	runs, err := ReadRuns(path)
	if err != nil {
		t.Fatalf("ReadRuns: %v", err)
	}
	if runs != nil {
		t.Fatalf("runs = %v, want nil", runs)
	}
}

func TestReadRunsEmptyFileReturnsNilNil(t *testing.T) {
	path := filepath.Join(t.TempDir(), "metrics.jsonl")
	if err := os.WriteFile(path, []byte{}, 0o644); err != nil {
		t.Fatalf("setup WriteFile: %v", err)
	}

	runs, err := ReadRuns(path)
	if err != nil {
		t.Fatalf("ReadRuns: %v", err)
	}
	if runs != nil {
		t.Fatalf("runs = %v, want nil", runs)
	}
}

func TestReadRunsValidFileReturnsRunsInOrder(t *testing.T) {
	path := filepath.Join(t.TempDir(), "metrics.jsonl")
	lines := []string{
		`{"schema_version":1,"timestamp":"2026-01-01T00:00:00Z","command":"add","duration_ms":10,"machine":{"os":"linux","arch":"amd64","num_cpu":4},"spans":[{"name":"copy","duration_ms":5}]}`,
		`{"schema_version":1,"timestamp":"2026-01-02T00:00:00Z","command":"add","duration_ms":20,"machine":{"os":"linux","arch":"amd64","num_cpu":4},"spans":[{"name":"copy","duration_ms":7}]}`,
		`{"schema_version":1,"timestamp":"2026-01-03T00:00:00Z","command":"sync","duration_ms":30,"machine":{"os":"linux","arch":"amd64","num_cpu":4},"spans":[]}`,
	}
	content := lines[0] + "\n" + lines[1] + "\n" + lines[2] + "\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("setup WriteFile: %v", err)
	}

	runs, err := ReadRuns(path)
	if err != nil {
		t.Fatalf("ReadRuns: %v", err)
	}
	if len(runs) != 3 {
		t.Fatalf("len(runs) = %d, want 3", len(runs))
	}

	wantTimestamps := []string{"2026-01-01T00:00:00Z", "2026-01-02T00:00:00Z", "2026-01-03T00:00:00Z"}
	for i, run := range runs {
		if run.Timestamp != wantTimestamps[i] {
			t.Errorf("runs[%d].Timestamp = %q, want %q", i, run.Timestamp, wantTimestamps[i])
		}
	}
	if runs[0].Command != cmdAdd || runs[2].Command != "sync" {
		t.Errorf("unexpected command order: %+v", runs)
	}
}

func TestReadRunsInvalidLineReturnsError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "metrics.jsonl")
	if err := os.WriteFile(path, []byte("not json\n"), 0o644); err != nil {
		t.Fatalf("setup WriteFile: %v", err)
	}

	if _, err := ReadRuns(path); err == nil {
		t.Fatal("expected error for invalid JSONL line")
	}
}

func TestReadRunsReadErrorPropagates(t *testing.T) {
	// A directory at path makes the underlying os.ReadFile fail with an
	// error other than "not exist", which ReadRuns must propagate rather
	// than swallow into (nil, nil).
	path := filepath.Join(t.TempDir(), "metrics.jsonl")
	if err := os.Mkdir(path, 0o755); err != nil {
		t.Fatalf("setup Mkdir: %v", err)
	}

	if _, err := ReadRuns(path); err == nil {
		t.Fatal("expected error when path is a directory")
	}
}

func TestPercentileEmptySliceReturnsZero(t *testing.T) {
	if got := percentile(nil, 0.5); got != 0 {
		t.Errorf("percentile(nil, 0.5) = %d, want 0", got)
	}
}

func TestPercentileClampsIndexBounds(t *testing.T) {
	sorted := []int64{10, 20}

	// p=0 drives idx to -1 before clamping up to 0.
	if got := percentile(sorted, 0); got != 10 {
		t.Errorf("percentile(sorted, 0) = %d, want 10 (clamped to index 0)", got)
	}
	// p=1.5 drives idx past len-1 before clamping down to len-1.
	if got := percentile(sorted, 1.5); got != 20 {
		t.Errorf("percentile(sorted, 1.5) = %d, want 20 (clamped to last index)", got)
	}
}

func TestMeanEmptySliceReturnsZero(t *testing.T) {
	if got := mean(nil); got != 0 {
		t.Errorf("mean(nil) = %d, want 0", got)
	}
}

func TestAggregateEmptySliceReturnsEmptyResult(t *testing.T) {
	reports := Aggregate(nil)
	if reports == nil {
		t.Fatal("Aggregate(nil) returned nil, want empty non-nil slice")
	}
	if len(reports) != 0 {
		t.Fatalf("len(reports) = %d, want 0", len(reports))
	}
}

// runWithSpans builds a fixture Run with the given command and one span per
// (name, durationMS) pair, ignoring every other Run field.
func runWithSpans(command string, spans ...Span) Run {
	return Run{Command: command, Spans: spans}
}

func span(name string, durationMS int64) Span {
	return Span{Name: name, DurationMS: durationMS}
}

func TestAggregateGroupsByCommandAndPhaseSortedDeterministically(t *testing.T) {
	runs := []Run{
		runWithSpans(cmdAdd, span("deps", 10), span("copy", 5)),
		runWithSpans(cmdAdd, span("deps", 20), span("copy", 7)),
		runWithSpans("sync", span("push", 100)),
	}

	reports := Aggregate(runs)

	if len(reports) != 2 {
		t.Fatalf("len(reports) = %d, want 2", len(reports))
	}
	// Commands sorted by name: "add" before "sync".
	if reports[0].Command != cmdAdd || reports[1].Command != "sync" {
		t.Fatalf("command order = [%s, %s], want [add, sync]", reports[0].Command, reports[1].Command)
	}
	if reports[0].Count != 2 {
		t.Errorf("reports[0].Count = %d, want 2", reports[0].Count)
	}
	// Phases sorted by name: "copy" before "deps".
	if len(reports[0].Phases) != 2 {
		t.Fatalf("len(phases) = %d, want 2", len(reports[0].Phases))
	}
	if reports[0].Phases[0].Name != "copy" || reports[0].Phases[1].Name != "deps" {
		t.Fatalf("phase order = [%s, %s], want [copy, deps]", reports[0].Phases[0].Name, reports[0].Phases[1].Name)
	}
}

func TestAggregatePercentilesOddSampleCount(t *testing.T) {
	// durations for phase "x": 10, 20, 30 (odd count = 3).
	runs := []Run{
		runWithSpans("cmd", span("x", 30)),
		runWithSpans("cmd", span("x", 10)),
		runWithSpans("cmd", span("x", 20)),
	}

	reports := Aggregate(runs)
	if len(reports) != 1 || len(reports[0].Phases) != 1 {
		t.Fatalf("unexpected shape: %+v", reports)
	}

	got := reports[0].Phases[0]
	want := PhaseStat{Name: "x", Count: 3, P50MS: 20, P95MS: 30, MeanMS: 20}
	if got != want {
		t.Errorf("phase stat = %+v, want %+v", got, want)
	}
}

func TestAggregatePercentilesEvenSampleCount(t *testing.T) {
	// durations for phase "x": 10, 20, 30, 40 (even count = 4).
	runs := []Run{
		runWithSpans("cmd", span("x", 10)),
		runWithSpans("cmd", span("x", 20)),
		runWithSpans("cmd", span("x", 30)),
		runWithSpans("cmd", span("x", 40)),
	}

	reports := Aggregate(runs)
	if len(reports) != 1 || len(reports[0].Phases) != 1 {
		t.Fatalf("unexpected shape: %+v", reports)
	}

	got := reports[0].Phases[0]
	want := PhaseStat{Name: "x", Count: 4, P50MS: 20, P95MS: 40, MeanMS: 25}
	if got != want {
		t.Errorf("phase stat = %+v, want %+v", got, want)
	}
}

func TestAggregatePercentilesSingleSample(t *testing.T) {
	runs := []Run{runWithSpans("cmd", span("x", 42))}

	reports := Aggregate(runs)
	if len(reports) != 1 || len(reports[0].Phases) != 1 {
		t.Fatalf("unexpected shape: %+v", reports)
	}

	got := reports[0].Phases[0]
	want := PhaseStat{Name: "x", Count: 1, P50MS: 42, P95MS: 42, MeanMS: 42}
	if got != want {
		t.Errorf("phase stat = %+v, want %+v", got, want)
	}
}

func TestAggregateDoesNotMutateInputOrder(t *testing.T) {
	runs := []Run{
		runWithSpans("zeta", span("x", 1)),
		runWithSpans("alpha", span("x", 2)),
		runWithSpans("mu", span("x", 3)),
	}
	before := make([]Run, len(runs))
	copy(before, runs)

	_ = Aggregate(runs)

	if !reflect.DeepEqual(runs, before) {
		t.Errorf("Aggregate mutated input order: got %+v, want %+v", runs, before)
	}
}

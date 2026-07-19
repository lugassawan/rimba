package observability

import (
	"bytes"
	"errors"
	"io"
	"os"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"
)

// fakeSink is a tiny in-memory test double — never the real fileSink,
// which sink_test.go exercises separately.
type fakeSink struct {
	mu      sync.Mutex
	logs    []any
	metrics []any
	closed  bool
}

func (f *fakeSink) WriteLog(record any) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.logs = append(f.logs, record)
	return nil
}

func (f *fakeSink) WriteMetric(record any) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.metrics = append(f.metrics, record)
	return nil
}

func (f *fakeSink) Close() error {
	f.closed = true
	return nil
}

func TestRecorderNilSafety(t *testing.T) {
	var r *Recorder

	r.LogSubprocess(CategoryGit, "somedir", []string{"status"}, 0, time.Millisecond, "some stderr", true)
	r.LogError("context", errors.New("boom"))

	stopSpan := r.StartSpan("x")
	stopSpan()

	stopModule := r.StartModuleSpan("somedir")
	stopModule(DetailClonedReflink)

	r.Finalize(OutcomeSuccess, 0, nil)

	if err := r.Close(); err != nil {
		t.Errorf("Close() on nil Recorder = %v, want nil", err)
	}
}

func TestMaybeDisabledOrNilSink(t *testing.T) {
	sink := &fakeSink{}

	if got := Maybe(false, sink, "add", "task", "svc", "v1"); got != nil {
		t.Errorf("Maybe(false, sink, ...) = %v, want nil", got)
	}
	if got := Maybe(true, nil, "add", "task", "svc", "v1"); got != nil {
		t.Errorf("Maybe(true, nil, ...) = %v, want nil", got)
	}
	if len(sink.logs) != 0 || len(sink.metrics) != 0 {
		t.Errorf("fakeSink recorded writes when Recorder should be nil: logs=%d metrics=%d", len(sink.logs), len(sink.metrics))
	}
}

func TestMaybeEnabledReturnsRecorder(t *testing.T) {
	sink := &fakeSink{}
	rec := Maybe(true, sink, "add", "task", "svc", "v1")
	if rec == nil {
		t.Fatal("Maybe(true, sink, ...) = nil, want non-nil Recorder")
	}
}

func TestFinalizeWritesOneCommandAndOneRootSpan(t *testing.T) {
	sink := &fakeSink{}
	rec := Maybe(true, sink, "add", "task", "svc", "v1")

	rec.Finalize(OutcomeSuccess, 0, nil)

	if len(sink.logs) != 1 {
		t.Fatalf("len(sink.logs) = %d, want 1", len(sink.logs))
	}
	if _, ok := sink.logs[0].(CommandRecord); !ok {
		t.Errorf("sink.logs[0] = %T, want CommandRecord", sink.logs[0])
	}
	if len(sink.metrics) != 1 {
		t.Fatalf("len(sink.metrics) = %d, want 1", len(sink.metrics))
	}
	span, ok := sink.metrics[0].(SpanRecord)
	if !ok {
		t.Fatalf("sink.metrics[0] = %T, want SpanRecord", sink.metrics[0])
	}
	if span.Name != "command" || span.ParentSpanID != "" {
		t.Errorf("root span = %+v, want Name=command, ParentSpanID empty", span)
	}
}

func TestStartSpanWritesChildSpan(t *testing.T) {
	sink := &fakeSink{}
	rec := Maybe(true, sink, "add", "task", "svc", "v1")

	stop := rec.StartSpan("x")
	time.Sleep(time.Millisecond)
	stop()

	if len(sink.metrics) != 1 {
		t.Fatalf("len(sink.metrics) = %d, want 1", len(sink.metrics))
	}
	span, ok := sink.metrics[0].(SpanRecord)
	if !ok {
		t.Fatalf("sink.metrics[0] = %T, want SpanRecord", sink.metrics[0])
	}
	if span.Name != "x" {
		t.Errorf("span.Name = %q, want %q", span.Name, "x")
	}
	if span.ParentSpanID != rec.rootSpanID {
		t.Errorf("span.ParentSpanID = %q, want root span ID %q", span.ParentSpanID, rec.rootSpanID)
	}
	if span.DurationMS < 0 {
		t.Errorf("span.DurationMS = %d, want non-negative", span.DurationMS)
	}
}

func TestStartModuleSpanDetail(t *testing.T) {
	tests := []struct {
		name       string
		wantDetail string
	}{
		{name: "cloned-reflink", wantDetail: DetailClonedReflink},
		{name: "cloned-copy", wantDetail: DetailClonedCopy},
		{name: "installed", wantDetail: DetailInstalled},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sink := &fakeSink{}
			rec := Maybe(true, sink, "add", "task", "svc", "v1")

			stop := rec.StartModuleSpan("frontend/node_modules")
			stop(tt.wantDetail)

			span, ok := sink.metrics[0].(SpanRecord)
			if !ok {
				t.Fatalf("sink.metrics[0] = %T, want SpanRecord", sink.metrics[0])
			}
			if span.Detail != tt.wantDetail {
				t.Errorf("span.Detail = %q, want %q", span.Detail, tt.wantDetail)
			}
			if span.Name != "deps:frontend/node_modules" {
				t.Errorf("span.Name = %q, want %q", span.Name, "deps:frontend/node_modules")
			}
		})
	}
}

// TestStartModuleSpanDisambiguatesSharedBasename verifies distinct modules
// that happen to share a basename (e.g. two "node_modules" dirs in different
// monorepo subdirs) get distinct span names — collapsing them under one name
// would average together installs with entirely different costs, hiding
// which one is actually slow.
func TestStartModuleSpanDisambiguatesSharedBasename(t *testing.T) {
	sink := &fakeSink{}
	rec := Maybe(true, sink, "add", "task", "svc", "v1")

	rec.StartModuleSpan("node_modules")(DetailInstalled)
	rec.StartModuleSpan("frontend/node_modules")(DetailInstalled)
	rec.StartModuleSpan("backend/node_modules")(DetailInstalled)

	if len(sink.metrics) != 3 {
		t.Fatalf("len(sink.metrics) = %d, want 3", len(sink.metrics))
	}

	seen := make(map[string]bool)
	for _, m := range sink.metrics {
		span, ok := m.(SpanRecord)
		if !ok {
			t.Fatalf("metric = %T, want SpanRecord", m)
		}
		seen[span.Name] = true
	}

	for _, want := range []string{"deps:node_modules", "deps:frontend/node_modules", "deps:backend/node_modules"} {
		if !seen[want] {
			t.Errorf("expected a span named %q, got names: %v", want, seen)
		}
	}
}

func TestLogSubprocessStderrOnlyOnFailure(t *testing.T) {
	sink := &fakeSink{}
	rec := Maybe(true, sink, "add", "task", "svc", "v1")

	rec.LogSubprocess(CategoryGit, "dir", []string{"status"}, 1, time.Millisecond, "some failure output", true)
	rec.LogSubprocess(CategoryGit, "dir", []string{"status"}, 0, time.Millisecond, "should be dropped", false)

	if len(sink.logs) != 2 {
		t.Fatalf("len(sink.logs) = %d, want 2", len(sink.logs))
	}
	failedRec, ok := sink.logs[0].(SubprocessRecord)
	if !ok {
		t.Fatalf("sink.logs[0] = %T, want SubprocessRecord", sink.logs[0])
	}
	if failedRec.Stderr != "some failure output" {
		t.Errorf("failed record Stderr = %q, want %q", failedRec.Stderr, "some failure output")
	}
	if failedRec.Outcome != OutcomeError {
		t.Errorf("failed record Outcome = %q, want %q", failedRec.Outcome, OutcomeError)
	}

	successRec, ok := sink.logs[1].(SubprocessRecord)
	if !ok {
		t.Fatalf("sink.logs[1] = %T, want SubprocessRecord", sink.logs[1])
	}
	if successRec.Stderr != "" {
		t.Errorf("success record Stderr = %q, want empty even though a stderr string was passed in", successRec.Stderr)
	}
	if successRec.Outcome != OutcomeSuccess {
		t.Errorf("success record Outcome = %q, want %q", successRec.Outcome, OutcomeSuccess)
	}
}

// TestLogErrorMessageTruncated confirms ErrorRecord.Message is capped the
// same way SubprocessRecord.Stderr already is — a failing post-create hook
// can wrap a very large combined stdout+stderr blob into its error.
func TestLogErrorMessageTruncated(t *testing.T) {
	sink := &fakeSink{}
	rec := Maybe(true, sink, "add", "task", "svc", "v1")

	huge := strings.Repeat("x", stderrTruncateLimit+500)
	rec.LogError("ctx", errors.New(huge))

	if len(sink.logs) != 1 {
		t.Fatalf("len(sink.logs) = %d, want 1", len(sink.logs))
	}
	errRec, ok := sink.logs[0].(ErrorRecord)
	if !ok {
		t.Fatalf("sink.logs[0] = %T, want ErrorRecord", sink.logs[0])
	}
	if !strings.HasSuffix(errRec.Message, "...(truncated)") {
		t.Errorf("Message = %q, want truncated suffix", errRec.Message)
	}
	if len(errRec.Message) >= len(huge) {
		t.Errorf("Message length = %d, want less than untruncated length %d", len(errRec.Message), len(huge))
	}
}

// TestFinalizeErrorTruncated confirms CommandRecord.Error is capped the same
// way — Finalize can receive an error built from a failing subprocess's full
// combined output.
func TestFinalizeErrorTruncated(t *testing.T) {
	sink := &fakeSink{}
	rec := Maybe(true, sink, "add", "task", "svc", "v1")

	huge := strings.Repeat("y", stderrTruncateLimit+500)
	rec.Finalize(OutcomeError, 1, errors.New(huge))

	if len(sink.logs) != 1 {
		t.Fatalf("len(sink.logs) = %d, want 1", len(sink.logs))
	}
	cmdRec, ok := sink.logs[0].(CommandRecord)
	if !ok {
		t.Fatalf("sink.logs[0] = %T, want CommandRecord", sink.logs[0])
	}
	if !strings.HasSuffix(cmdRec.Error, "...(truncated)") {
		t.Errorf("Error = %q, want truncated suffix", cmdRec.Error)
	}
	if len(cmdRec.Error) >= len(huge) {
		t.Errorf("Error length = %d, want less than untruncated length %d", len(cmdRec.Error), len(huge))
	}
}

func TestRunIDStableSeqIncreasing(t *testing.T) {
	sink := &fakeSink{}
	rec := Maybe(true, sink, "add", "task", "svc", "v1")

	rec.LogSubprocess(CategoryGit, "dir", []string{"status"}, 0, time.Millisecond, "", false)
	rec.LogError("ctx", errors.New("boom"))
	stop := rec.StartSpan("x")
	stop()
	rec.Finalize(OutcomeSuccess, 0, nil)

	var runIDs []string
	var seqs []uint64

	collect := func(runID string, seq uint64) {
		runIDs = append(runIDs, runID)
		seqs = append(seqs, seq)
	}

	for _, rawLog := range sink.logs {
		switch v := rawLog.(type) {
		case SubprocessRecord:
			collect(v.RunID, v.Seq)
		case ErrorRecord:
			collect(v.RunID, v.Seq)
		case CommandRecord:
			collect(v.RunID, v.Seq)
		}
	}
	for _, rawMetric := range sink.metrics {
		if v, ok := rawMetric.(SpanRecord); ok {
			collect(v.RunID, v.Seq)
		}
	}

	for _, id := range runIDs {
		if id != rec.runID {
			t.Errorf("record run_id = %q, want %q (all records from one Recorder share run_id)", id, rec.runID)
		}
	}

	// Seq is assigned in actual call order (LogSubprocess, LogError, the
	// StartSpan closure, then Finalize's CommandRecord + root span), which
	// does not match the order records land in sink.logs vs sink.metrics
	// (log-stream records first, then metric-stream records above). Sort
	// before checking strict monotonicity so this asserts the real
	// invariant — every Seq is unique and increasing — not an artifact of
	// which stream we happened to collect first.
	slices.Sort(seqs)
	for i := 1; i < len(seqs); i++ {
		if seqs[i] <= seqs[i-1] {
			t.Errorf("seqs not strictly increasing once sorted: %v", seqs)
		}
	}
}

func TestFinalizeWithErrorPopulatesErrorField(t *testing.T) {
	sink := &fakeSink{}
	rec := Maybe(true, sink, "add", "task", "svc", "v1")

	rec.Finalize(OutcomeError, 1, errors.New("boom"))

	cmdRec, ok := sink.logs[0].(CommandRecord)
	if !ok {
		t.Fatalf("sink.logs[0] = %T, want CommandRecord", sink.logs[0])
	}
	if cmdRec.Error != "boom" {
		t.Errorf("cmdRec.Error = %q, want %q", cmdRec.Error, "boom")
	}
}

func TestLogSubprocessDebugTimingWritesToStderr(t *testing.T) {
	t.Setenv("RIMBA_DEBUG", "1")
	sink := &fakeSink{}
	rec := Maybe(true, sink, "add", "task", "svc", "v1")

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	origStderr := os.Stderr
	os.Stderr = w

	rec.LogSubprocess(CategoryGit, "somedir", []string{"status"}, 0, time.Millisecond, "", false)

	if cerr := w.Close(); cerr != nil {
		t.Fatalf("closing pipe writer: %v", cerr)
	}
	os.Stderr = origStderr

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		t.Fatalf("reading pipe: %v", err)
	}
	if !strings.Contains(buf.String(), "[debug]") {
		t.Errorf("stderr output = %q, want it to contain %q", buf.String(), "[debug]")
	}
}

func TestRecorderConcurrentLogSubprocessWithRealFileSink(t *testing.T) {
	cacheDir := t.TempDir()
	sink, err := newFileSinkAt(cacheDir, "/repo/recorder-concurrency", 14)
	if err != nil {
		t.Fatalf("newFileSinkAt: %v", err)
	}
	rec := Maybe(true, sink, "add", "task", "svc", "v1")

	const goroutines = 4
	const writesEach = 50
	var wg sync.WaitGroup
	wg.Add(goroutines)
	for range goroutines {
		go func() {
			defer wg.Done()
			for range writesEach {
				rec.LogSubprocess(CategoryGit, "dir", []string{"status"}, 0, time.Microsecond, "", false)
			}
		}()
	}
	wg.Wait()

	if err := rec.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}

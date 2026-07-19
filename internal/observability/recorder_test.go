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
	stopModule(true)

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
		cloned     bool
		wantDetail string
	}{
		{name: "cloned", cloned: true, wantDetail: "cloned"},
		{name: "installed", cloned: false, wantDetail: "installed"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sink := &fakeSink{}
			rec := Maybe(true, sink, "add", "task", "svc", "v1")

			stop := rec.StartModuleSpan("/path/to/mymodule")
			stop(tt.cloned)

			span, ok := sink.metrics[0].(SpanRecord)
			if !ok {
				t.Fatalf("sink.metrics[0] = %T, want SpanRecord", sink.metrics[0])
			}
			if span.Detail != tt.wantDetail {
				t.Errorf("span.Detail = %q, want %q", span.Detail, tt.wantDetail)
			}
			if span.Name != "deps:mymodule" {
				t.Errorf("span.Name = %q, want %q", span.Name, "deps:mymodule")
			}
		})
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

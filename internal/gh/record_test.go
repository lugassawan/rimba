package gh

import (
	"context"
	"errors"
	"testing"

	"github.com/lugassawan/rimba/internal/observability"
)

// fakeSink is a tiny in-memory observability.Sink test double.
type fakeSink struct {
	logs []any
}

func (f *fakeSink) WriteLog(record any) error {
	f.logs = append(f.logs, record)
	return nil
}

func (f *fakeSink) WriteMetric(_ any) error {
	return nil
}

func (f *fakeSink) Close() error {
	return nil
}

func TestWrapRunnerRecordsSuccess(t *testing.T) {
	sink := &fakeSink{}
	rec := observability.Maybe(true, sink, "add", "", "", "v1")
	inner := &mockRunner{run: func(_ context.Context, _ ...string) ([]byte, error) {
		return []byte("out"), nil
	}}

	wrapped := WrapRunner(inner)
	out, err := wrapped.Run(observability.WithRecorder(context.Background(), rec), "pr", "view")
	if err != nil || string(out) != "out" {
		t.Fatalf("Run() = %q, %v, want %q, nil", out, err, "out")
	}

	if len(sink.logs) != 1 {
		t.Fatalf("len(sink.logs) = %d, want 1", len(sink.logs))
	}
	subRec, ok := sink.logs[0].(observability.SubprocessRecord)
	if !ok {
		t.Fatalf("sink.logs[0] = %T, want SubprocessRecord", sink.logs[0])
	}
	if subRec.Category != observability.CategoryGH {
		t.Errorf("Category = %q, want %q", subRec.Category, observability.CategoryGH)
	}
	if len(subRec.Args) != 2 || subRec.Args[0] != "pr" || subRec.Args[1] != "view" {
		t.Errorf("Args = %v, want [pr view]", subRec.Args)
	}
	if subRec.Outcome != observability.OutcomeSuccess {
		t.Errorf("Outcome = %q, want %q", subRec.Outcome, observability.OutcomeSuccess)
	}
	if subRec.ExitCode != 0 {
		t.Errorf("ExitCode = %d, want 0", subRec.ExitCode)
	}
}

func TestWrapRunnerRecordsError(t *testing.T) {
	sink := &fakeSink{}
	rec := observability.Maybe(true, sink, "add", "", "", "v1")
	inner := &mockRunner{run: func(_ context.Context, _ ...string) ([]byte, error) {
		return nil, errors.New("boom")
	}}

	wrapped := WrapRunner(inner)
	_, err := wrapped.Run(observability.WithRecorder(context.Background(), rec), "pr", "create")
	if err == nil {
		t.Fatal("expected error to propagate")
	}

	if len(sink.logs) != 1 {
		t.Fatalf("len(sink.logs) = %d, want 1", len(sink.logs))
	}
	subRec, ok := sink.logs[0].(observability.SubprocessRecord)
	if !ok {
		t.Fatalf("sink.logs[0] = %T, want SubprocessRecord", sink.logs[0])
	}
	if subRec.Outcome != observability.OutcomeError {
		t.Errorf("Outcome = %q, want %q", subRec.Outcome, observability.OutcomeError)
	}
	if subRec.ExitCode != -1 {
		t.Errorf("ExitCode = %d, want -1", subRec.ExitCode)
	}
	if subRec.Stderr != "boom" {
		t.Errorf("Stderr = %q, want %q", subRec.Stderr, "boom")
	}
}

// TestWrapRunnerNoRecorderNoPanic confirms a call whose ctx carries no
// Recorder is a silent no-op — never a panic, never a record.
func TestWrapRunnerNoRecorderNoPanic(t *testing.T) {
	inner := &mockRunner{run: func(_ context.Context, _ ...string) ([]byte, error) {
		return []byte("out"), nil
	}}

	wrapped := WrapRunner(inner)
	out, err := wrapped.Run(context.Background(), "pr", "view")
	if err != nil || string(out) != "out" {
		t.Fatalf("Run() = %q, %v, want %q, nil", out, err, "out")
	}
}

// TestWrapRunnerDerivesRecorderPerCall is the MCP-fix regression test: one
// long-lived wrapped Runner instance (built once, as MCP's HandlerContext
// holds it for the server's whole lifetime) must record each call according
// to that call's OWN ctx, not a Recorder captured once when the Runner was
// constructed.
func TestWrapRunnerDerivesRecorderPerCall(t *testing.T) {
	inner := &mockRunner{run: func(_ context.Context, _ ...string) ([]byte, error) {
		return []byte("out"), nil
	}}
	wrapped := WrapRunner(inner) // constructed once, like MCP's HandlerContext.GH

	sinkA := &fakeSink{}
	recA := observability.Maybe(true, sinkA, "add", "", "", "v1")
	if _, err := wrapped.Run(observability.WithRecorder(context.Background(), recA), "pr", "view"); err != nil {
		t.Fatalf("call A: %v", err)
	}

	if _, err := wrapped.Run(context.Background(), "pr", "view"); err != nil {
		t.Fatalf("call B (no recorder): %v", err)
	}

	sinkC := &fakeSink{}
	recC := observability.Maybe(true, sinkC, "merge-plan", "", "", "v1")
	if _, err := wrapped.Run(observability.WithRecorder(context.Background(), recC), "pr", "view"); err != nil {
		t.Fatalf("call C: %v", err)
	}

	if len(sinkA.logs) != 1 {
		t.Errorf("sinkA.logs = %d, want 1 (call A's own record only)", len(sinkA.logs))
	}
	if len(sinkC.logs) != 1 {
		t.Errorf("sinkC.logs = %d, want 1 (call C's own record only)", len(sinkC.logs))
	}
}

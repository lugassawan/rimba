package mcp

import (
	"context"
	"errors"
	"runtime"
	"strings"
	"testing"

	"github.com/lugassawan/rimba/internal/config"
	"github.com/lugassawan/rimba/internal/observability"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

var errBoom = errors.New("handler boom")

// recordingHandler returns a server.ToolHandlerFunc that records whether a
// Recorder was attached to ctx, then returns result.
func recordingHandler(sawRecorder *bool, result *mcp.CallToolResult) server.ToolHandlerFunc {
	return func(ctx context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		*sawRecorder = observability.FromContext(ctx) != nil
		return result, nil
	}
}

func TestWithRecorderNilSinkSkipsWrapping(t *testing.T) {
	hctx := &HandlerContext{Config: nil, RepoRoot: "/repo", Version: "test"}
	var sawRecorder bool
	handler := withRecorder(hctx, "add", recordingHandler(&sawRecorder, mcp.NewToolResultText("ok")))

	result, err := handler(context.Background(), mcp.CallToolRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sawRecorder {
		t.Error("expected no Recorder attached when Sink is nil")
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
}

func TestWithRecorderEnabledConfigNilSinkSkipsWrapping(t *testing.T) {
	hctx := &HandlerContext{Config: &config.Config{}, RepoRoot: "/repo", Version: "test"}
	var sawRecorder bool
	handler := withRecorder(hctx, "add", recordingHandler(&sawRecorder, mcp.NewToolResultText("ok")))

	if _, err := handler(context.Background(), mcp.CallToolRequest{}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sawRecorder {
		t.Error("expected no Recorder attached when Sink is nil, even with observability enabled")
	}
}

func TestWithRecorderAttachesRecorderAndWritesCommandRecord(t *testing.T) {
	sink := &fakeSink{}
	hctx := &HandlerContext{Sink: sink, RepoRoot: "/repo", Version: "test"}
	var sawRecorder bool
	handler := withRecorder(hctx, "add", recordingHandler(&sawRecorder, mcp.NewToolResultText("ok")))

	if _, err := handler(context.Background(), mcp.CallToolRequest{}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !sawRecorder {
		t.Fatal("expected a Recorder attached to ctx when a Sink is set")
	}
	rec, ok := findCommandRecord(sink, func(r observability.CommandRecord) bool { return r.Command == "add" })
	if !ok {
		t.Fatalf("no CommandRecord for command %q in %v", "add", sink.logs)
	}
	if rec.Outcome != observability.OutcomeSuccess {
		t.Errorf("outcome = %q, want %q", rec.Outcome, observability.OutcomeSuccess)
	}
}

// TestWithRecorderSharedSinkSurvivesAcrossCalls guards the shared-sink
// contract: the per-call Recorder must never close the sink, or every call
// after the first would silently lose its records.
func TestWithRecorderSharedSinkSurvivesAcrossCalls(t *testing.T) {
	sink := &fakeSink{}
	hctx := &HandlerContext{Sink: sink, RepoRoot: "/repo", Version: "test"}
	var sawRecorder bool
	handler := withRecorder(hctx, "add", recordingHandler(&sawRecorder, mcp.NewToolResultText("ok")))

	for range 2 {
		if _, err := handler(context.Background(), mcp.CallToolRequest{}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	}

	commands := 0
	for _, l := range sink.logs {
		if r, ok := asCommandRecord(l); ok && r.Command == "add" {
			commands++
		}
	}
	if commands != 2 {
		t.Errorf("command records = %d, want 2", commands)
	}
	if sink.closes != 0 {
		t.Errorf("sink closed %d times by withRecorder, want 0", sink.closes)
	}
}

func TestWithRecorderErrorResultMarksOutcomeError(t *testing.T) {
	sink := &fakeSink{}
	hctx := &HandlerContext{Sink: sink, RepoRoot: "/repo", Version: "test"}
	var sawRecorder bool
	handler := withRecorder(hctx, "remove", recordingHandler(&sawRecorder, errorResult(errors.New("boom"))))

	if _, err := handler(context.Background(), mcp.CallToolRequest{}); err != nil {
		t.Fatalf("unexpected protocol error: %v", err)
	}
	if _, ok := findCommandRecord(sink, func(r observability.CommandRecord) bool {
		return r.Outcome == observability.OutcomeError
	}); !ok {
		t.Errorf("no error-outcome CommandRecord in %v", sink.logs)
	}
}

// asCommandRecord unwraps a sink log entry, which Recorder may write by value or pointer.
func asCommandRecord(v any) (observability.CommandRecord, bool) {
	switch r := v.(type) {
	case observability.CommandRecord:
		return r, true
	case *observability.CommandRecord:
		return *r, true
	}
	return observability.CommandRecord{}, false
}

// findCommandRecord returns the first CommandRecord in sink's log stream satisfying match.
func findCommandRecord(sink *fakeSink, match func(observability.CommandRecord) bool) (observability.CommandRecord, bool) {
	for _, l := range sink.logs {
		if r, ok := asCommandRecord(l); ok && match(r) {
			return r, true
		}
	}
	return observability.CommandRecord{}, false
}

// callRecoveringPanic invokes h and returns the value it panicked with.
func callRecoveringPanic(t *testing.T, h server.ToolHandlerFunc) (recovered any) {
	t.Helper()
	defer func() { recovered = recover() }()
	_, _ = h(context.Background(), mcp.CallToolRequest{})
	return nil
}

func TestWithRecorderPanicRecordsErrorAndRepanics(t *testing.T) {
	sink := &fakeSink{}
	hctx := &HandlerContext{Sink: sink, RepoRoot: "/repo", Version: "test"}
	handler := withRecorder(hctx, "add", func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		panic(errBoom)
	})

	if got := callRecoveringPanic(t, handler); got != errBoom { //nolint:errorlint // asserting identity of the re-panicked value
		t.Fatalf("panic value = %v, want errBoom", got)
	}

	if _, ok := findCommandRecord(sink, func(r observability.CommandRecord) bool {
		return r.Outcome == observability.OutcomeError && strings.Contains(r.Error, "panic:")
	}); !ok {
		t.Errorf("no error CommandRecord with a panic message in %v", sink.logs)
	}
	if len(sink.metrics) == 0 {
		t.Error("expected a root command span in the metrics stream")
	}
}

func TestWithRecorderGoexitRecordsErrorNotSuccess(t *testing.T) {
	sink := &fakeSink{}
	hctx := &HandlerContext{Sink: sink, RepoRoot: "/repo", Version: "test"}
	handler := withRecorder(hctx, "add", func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		runtime.Goexit()
		return nil, errBoom // unreachable
	})

	done := make(chan struct{})
	go func() {
		defer close(done)
		_, _ = handler(context.Background(), mcp.CallToolRequest{})
	}()
	<-done

	if _, ok := findCommandRecord(sink, func(r observability.CommandRecord) bool {
		return r.Outcome == observability.OutcomeSuccess
	}); ok {
		t.Errorf("Goexit was recorded as success: %v", sink.logs)
	}
	if _, ok := findCommandRecord(sink, func(r observability.CommandRecord) bool {
		return r.Outcome == observability.OutcomeError
	}); !ok {
		t.Errorf("expected an error CommandRecord for Goexit: %v", sink.logs)
	}
}

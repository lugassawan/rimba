package deps

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lugassawan/rimba/internal/observability"
)

func TestRunPostCreateHooksSuccess(t *testing.T) {
	dir := t.TempDir()

	results := RunPostCreateHooks(context.Background(), dir, []string{"touch marker.txt"}, nil)

	if len(results) != 1 {
		t.Fatalf(fmtExpectedOneResult, len(results))
	}

	if results[0].Error != nil {
		t.Errorf(fmtExpectedNoError, results[0].Error)
	}

	if _, err := os.Stat(filepath.Join(dir, "marker.txt")); os.IsNotExist(err) {
		t.Error("expected marker.txt to exist")
	}
}

func TestRunPostCreateHooksPartialFailure(t *testing.T) {
	dir := t.TempDir()

	results := RunPostCreateHooks(context.Background(), dir, []string{
		"touch good.txt",
		"false", // always fails
		"touch also-good.txt",
	}, nil)

	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}

	if results[0].Error != nil {
		t.Error("expected first hook to succeed")
	}
	if results[1].Error == nil {
		t.Error("expected second hook to fail")
	}
	if results[2].Error != nil {
		t.Error("expected third hook to succeed")
	}

	// Both good hooks should have run
	if _, err := os.Stat(filepath.Join(dir, "good.txt")); os.IsNotExist(err) {
		t.Error("expected good.txt to exist")
	}
	if _, err := os.Stat(filepath.Join(dir, "also-good.txt")); os.IsNotExist(err) {
		t.Error("expected also-good.txt to exist")
	}
}

func TestRunPostCreateHooksEmpty(t *testing.T) {
	dir := t.TempDir()

	results := RunPostCreateHooks(context.Background(), dir, nil, nil)

	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
}

func TestRunPostCreateHooksShellFeatures(t *testing.T) {
	dir := t.TempDir()

	// Test shell features: pipes and quoting
	results := RunPostCreateHooks(context.Background(), dir, []string{
		"echo 'hello world' > output.txt",
	}, nil)

	if results[0].Error != nil {
		t.Fatalf(fmtExpectedNoError, results[0].Error)
	}

	data, err := os.ReadFile(filepath.Join(dir, "output.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "hello world\n" {
		t.Errorf("expected 'hello world\\n', got %q", string(data))
	}
}

func TestRunPostCreateHooksProgressCallback(t *testing.T) {
	dir := t.TempDir()

	var calls []progressCall
	onProgress := func(msg string) {
		calls = append(calls, progressCall{msg})
	}

	hooks := []string{"touch a.txt", "touch b.txt"}
	RunPostCreateHooks(context.Background(), dir, hooks, onProgress)

	if len(calls) != 2 {
		t.Fatalf("expected 2 progress calls, got %d", len(calls))
	}
	if want := "touch a.txt (1/2)"; calls[0].message != want {
		t.Errorf("calls[0] = %q, want %q", calls[0].message, want)
	}
	if want := "touch b.txt (2/2)"; calls[1].message != want {
		t.Errorf("calls[1] = %q, want %q", calls[1].message, want)
	}
}

func TestRunPostCreateHooksOutputCapture(t *testing.T) {
	dir := t.TempDir()

	results := RunPostCreateHooks(context.Background(), dir, []string{
		"echo hook-output-captured && exit 1",
	}, nil)

	if len(results) != 1 {
		t.Fatalf(fmtExpectedOneResult, len(results))
	}
	if results[0].Error == nil {
		t.Fatal("expected error from failing hook")
	}
	if !strings.Contains(results[0].Error.Error(), "hook-output-captured") {
		t.Errorf("error should contain captured output, got %q", results[0].Error.Error())
	}
}

func TestRunPostCreateHooksOutputTailCapped(t *testing.T) {
	// Regression test for the bounded tail buffer: a failing hook that writes
	// more than outputTailCapBytes of output must produce an error message
	// containing only the tail — the earliest-written marker should be
	// dropped, the latest-written marker must survive.
	dir := t.TempDir()
	fillerSize := outputTailCapBytes + 4096
	// The markers are split across separate printf calls in the hook source
	// so the contiguous marker text only ever appears in the captured
	// *output*, never in the hook string itself — RunPostCreateHooks embeds
	// the raw hook string (via %q) in the wrapped error, which would
	// otherwise make this assertion pass regardless of whether output
	// capping works.
	hook := fmt.Sprintf(
		"printf 'EARLY'; printf 'MARKER'; yes x | head -c %d; printf 'LATE'; printf 'MARKERTAIL'; exit 1",
		fillerSize)

	results := RunPostCreateHooks(context.Background(), dir, []string{hook}, nil)

	if len(results) != 1 {
		t.Fatalf(fmtExpectedOneResult, len(results))
	}
	if results[0].Error == nil {
		t.Fatal("expected error from failing hook")
	}
	if strings.Contains(results[0].Error.Error(), "EARLYMARKER") {
		t.Error("error contains the earliest-written output, want it dropped by the tail cap")
	}
	if !strings.Contains(results[0].Error.Error(), "LATEMARKERTAIL") {
		t.Error("error should contain the latest-written output")
	}
}

// TestRunPostCreateHooksRecordsSubprocess verifies each hook invocation is
// recorded via the Recorder attached to ctx, with the right category/exit
// code/outcome.
func TestRunPostCreateHooksRecordsSubprocess(t *testing.T) {
	dir := t.TempDir()
	sink := &fakeSink{}
	rec := observability.Maybe(true, sink, "add", "task", "", "v1")
	ctx := observability.WithRecorder(context.Background(), rec)

	results := RunPostCreateHooks(ctx, dir, []string{"touch good.txt", "exit 3"}, nil)
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}

	if len(sink.logs) != 2 {
		t.Fatalf("len(sink.logs) = %d, want 2", len(sink.logs))
	}

	okRec, ok := sink.logs[0].(observability.SubprocessRecord)
	if !ok {
		t.Fatalf("sink.logs[0] = %T, want SubprocessRecord", sink.logs[0])
	}
	if okRec.Category != observability.CategoryHook {
		t.Errorf("Category = %q, want %q", okRec.Category, observability.CategoryHook)
	}
	if okRec.Outcome != observability.OutcomeSuccess || okRec.ExitCode != 0 {
		t.Errorf("first hook record = %+v, want success/exit 0", okRec)
	}

	failRec, ok := sink.logs[1].(observability.SubprocessRecord)
	if !ok {
		t.Fatalf("sink.logs[1] = %T, want SubprocessRecord", sink.logs[1])
	}
	if failRec.Outcome != observability.OutcomeError || failRec.ExitCode != 3 {
		t.Errorf("second hook record = %+v, want error/exit 3", failRec)
	}
}

// TestRunPostCreateHooksNilRecorderNoPanic confirms observability-off callers
// (bare context.Background(), no Recorder attached) still work — the
// nil-safety contract exercised in practice, not just in isolation.
func TestRunPostCreateHooksNilRecorderNoPanic(t *testing.T) {
	dir := t.TempDir()

	results := RunPostCreateHooks(context.Background(), dir, []string{"touch marker.txt"}, nil)

	if len(results) != 1 || results[0].Error != nil {
		t.Fatalf("expected 1 successful result, got %+v", results)
	}
}

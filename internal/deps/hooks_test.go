package deps

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/lugassawan/rimba/internal/observability"
)

func TestRunPostCreateHooksSuccess(t *testing.T) {
	dir := t.TempDir()

	results := RunPostCreateHooks(context.Background(), dir, [][]string{{"touch marker.txt"}}, nil)

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

	results := RunPostCreateHooks(context.Background(), dir, serialStages([]string{
		"touch good.txt",
		"false", // always fails
		"touch also-good.txt",
	}), nil)

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
	results := RunPostCreateHooks(context.Background(), dir, [][]string{{
		"echo 'hello world' > output.txt",
	}}, nil)

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
	RunPostCreateHooks(context.Background(), dir, serialStages(hooks), onProgress)

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

	results := RunPostCreateHooks(context.Background(), dir, [][]string{{
		"echo hook-output-captured && exit 1",
	}}, nil)

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

	results := RunPostCreateHooks(context.Background(), dir, [][]string{{hook}}, nil)

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

	results := RunPostCreateHooks(ctx, dir, serialStages([]string{"touch good.txt", "exit 3"}), nil)
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

	results := RunPostCreateHooks(context.Background(), dir, [][]string{{"touch marker.txt"}}, nil)

	if len(results) != 1 || results[0].Error != nil {
		t.Fatalf("expected 1 successful result, got %+v", results)
	}
}

// --- multi-command stages (a single stage holding more than one command) ---

func TestRunPostCreateHooksParallelAllRun(t *testing.T) {
	dir := t.TempDir()

	// Each hook touches a distinct file; all files existing afterward proves
	// every hook ran even though execution order is not guaranteed.
	hooks := []string{
		"touch one.txt",
		"touch two.txt",
		"touch three.txt",
	}

	results := RunPostCreateHooks(context.Background(), dir, [][]string{hooks}, nil)
	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}
	for _, name := range []string{"one.txt", "two.txt", "three.txt"} {
		if _, err := os.Stat(filepath.Join(dir, name)); os.IsNotExist(err) {
			t.Errorf("expected %s to exist", name)
		}
	}
	// parallel.Collect preserves input order in the output slice.
	for i, hook := range hooks {
		if results[i].Command != hook {
			t.Errorf("results[%d].Command = %q, want %q (order not preserved)", i, results[i].Command, hook)
		}
	}
}

func TestRunPostCreateHooksParallelFailureIsolation(t *testing.T) {
	dir := t.TempDir()

	// Mirrors serial's continue-past-failure semantics: a failing hook must
	// not prevent the others from running.
	hooks := []string{
		"touch good-a.txt",
		"false", // always fails
		"touch good-b.txt",
	}

	results := RunPostCreateHooks(context.Background(), dir, [][]string{hooks}, nil)
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
	if _, err := os.Stat(filepath.Join(dir, "good-a.txt")); os.IsNotExist(err) {
		t.Error("expected good-a.txt to exist")
	}
	if _, err := os.Stat(filepath.Join(dir, "good-b.txt")); os.IsNotExist(err) {
		t.Error("expected good-b.txt to exist")
	}
}

func TestRunPostCreateHooksParallelRecordsSubprocessPerHook(t *testing.T) {
	dir := t.TempDir()
	sink := &fakeSink{}
	rec := observability.Maybe(true, sink, "add", "task", "", "v1")
	ctx := observability.WithRecorder(context.Background(), rec)

	hooks := []string{"touch a.txt", "touch b.txt", "false"}

	results := RunPostCreateHooks(ctx, dir, [][]string{hooks}, nil)
	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}

	if len(sink.logs) != 3 {
		t.Fatalf("len(sink.logs) = %d, want 3", len(sink.logs))
	}
	// Log order need not match hook order in parallel mode (concurrent
	// LogSubprocess calls race), so assert on the set of records, not positions.
	byHook := make(map[string]observability.SubprocessRecord, len(sink.logs))
	for _, l := range sink.logs {
		subRec, ok := l.(observability.SubprocessRecord)
		if !ok {
			t.Fatalf("sink.logs entry = %T, want SubprocessRecord", l)
		}
		if len(subRec.Args) != 1 {
			t.Fatalf("Args = %v, want exactly 1 hook", subRec.Args)
		}
		byHook[subRec.Args[0]] = subRec
	}
	for _, hook := range hooks {
		got, ok := byHook[hook]
		if !ok {
			t.Errorf("expected a subprocess record for %q, got %+v", hook, sink.logs)
			continue
		}
		if got.Category != observability.CategoryHook {
			t.Errorf("Category = %q, want %q", got.Category, observability.CategoryHook)
		}
	}
	if got := byHook["false"]; got.Outcome != observability.OutcomeError {
		t.Errorf("expected the failing hook to record %q, got %q", observability.OutcomeError, got.Outcome)
	}
}

func TestRunPostCreateHooksParallelProgressCallback(t *testing.T) {
	dir := t.TempDir()

	var mu sync.Mutex
	var calls []string
	onProgress := func(msg string) {
		mu.Lock()
		defer mu.Unlock()
		calls = append(calls, msg)
	}

	hooks := []string{"touch a.txt", "touch b.txt", "touch c.txt"}
	RunPostCreateHooks(context.Background(), dir, [][]string{hooks}, onProgress)

	mu.Lock()
	defer mu.Unlock()
	if len(calls) != 3 {
		t.Fatalf("expected 3 progress calls, got %d", len(calls))
	}
	// A multi-command stage reports a completion count ("N/total complete"),
	// not the single-command-stage "hook (i/N)" ordinal message — ordinals
	// don't mean the same thing once commands run concurrently.
	want := map[string]bool{
		"1/3 complete": true,
		"2/3 complete": true,
		"3/3 complete": true,
	}
	for _, c := range calls {
		if !want[c] {
			t.Errorf("unexpected progress message %q", c)
		}
		delete(want, c)
	}
	if len(want) != 0 {
		t.Errorf("missing progress messages: %v", want)
	}
}

func TestRunPostCreateHooksParallelIsActuallyConcurrent(t *testing.T) {
	// Proves genuine concurrency (not just "all hooks ran eventually") via
	// timing: 5 hooks each sleeping ~150ms should complete in well under
	// 5*150ms if truly running in parallel.
	dir := t.TempDir()

	const n = 5
	const sleepMS = 150
	hooks := make([]string, n)
	for i := range hooks {
		hooks[i] = fmt.Sprintf("sleep %.3f", sleepMS/1000.0)
	}

	start := time.Now()
	results := RunPostCreateHooks(context.Background(), dir, [][]string{hooks}, nil)
	elapsed := time.Since(start)

	if len(results) != n {
		t.Fatalf("expected %d results, got %d", n, len(results))
	}
	for i, r := range results {
		if r.Error != nil {
			t.Errorf("results[%d]: unexpected error %v", i, r.Error)
		}
	}

	serialWorstCase := n * sleepMS * time.Millisecond
	if elapsed >= serialWorstCase {
		t.Errorf("elapsed %v was not faster than serial worst case %v — hooks do not appear to run concurrently", elapsed, serialWorstCase)
	}
}

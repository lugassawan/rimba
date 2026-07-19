package observability

import (
	"context"
	"errors"
	"io"
	"os"
	"strings"
	"testing"
)

// stubRunnerOutput is the fixed stdout stubRunner returns from both methods.
const stubRunnerOutput = "out"

// stubRunner is a minimal git.Runner test double, mirroring internal/debug's.
type stubRunner struct {
	runErr      error
	runInDirErr error
}

func (s *stubRunner) Run(_ context.Context, _ ...string) (string, error) {
	return stubRunnerOutput, s.runErr
}

func (s *stubRunner) RunInDir(_ context.Context, _ string, _ ...string) (string, error) {
	return stubRunnerOutput, s.runInDirErr
}

func TestWrapRunnerRunRecordsSuccess(t *testing.T) {
	sink := &fakeSink{}
	rec := Maybe(true, sink, "add", "task", "svc", "v1")
	stub := &stubRunner{}

	wrapped := WrapRunner(stub)
	out, err := wrapped.Run(WithRecorder(context.Background(), rec), "status", "-s")
	if err != nil || out != stubRunnerOutput {
		t.Fatalf("Run() = %q, %v, want %q, nil", out, err, stubRunnerOutput)
	}

	if len(sink.logs) != 1 {
		t.Fatalf("len(sink.logs) = %d, want 1", len(sink.logs))
	}
	rec2, ok := sink.logs[0].(SubprocessRecord)
	if !ok {
		t.Fatalf("sink.logs[0] = %T, want SubprocessRecord", sink.logs[0])
	}
	if rec2.Category != CategoryGit {
		t.Errorf("Category = %q, want %q", rec2.Category, CategoryGit)
	}
	if len(rec2.Args) != 2 || rec2.Args[0] != "status" || rec2.Args[1] != "-s" {
		t.Errorf("Args = %v, want [status -s]", rec2.Args)
	}
	if rec2.Dir != "" {
		t.Errorf("Dir = %q, want empty", rec2.Dir)
	}
	if rec2.Outcome != OutcomeSuccess {
		t.Errorf("Outcome = %q, want %q", rec2.Outcome, OutcomeSuccess)
	}
	if rec2.ExitCode != 0 {
		t.Errorf("ExitCode = %d, want 0", rec2.ExitCode)
	}
}

func TestWrapRunnerRunInDirRecordsError(t *testing.T) {
	sink := &fakeSink{}
	rec := Maybe(true, sink, "add", "task", "svc", "v1")
	stub := &stubRunner{runInDirErr: errors.New("boom")}

	wrapped := WrapRunner(stub)
	_, err := wrapped.RunInDir(WithRecorder(context.Background(), rec), "/tmp/worktree", "fetch")
	if err == nil {
		t.Fatal("expected error to propagate")
	}

	if len(sink.logs) != 1 {
		t.Fatalf("len(sink.logs) = %d, want 1", len(sink.logs))
	}
	rec2, ok := sink.logs[0].(SubprocessRecord)
	if !ok {
		t.Fatalf("sink.logs[0] = %T, want SubprocessRecord", sink.logs[0])
	}
	if rec2.Dir != "/tmp/worktree" {
		t.Errorf("Dir = %q, want %q", rec2.Dir, "/tmp/worktree")
	}
	if rec2.Outcome != OutcomeError {
		t.Errorf("Outcome = %q, want %q", rec2.Outcome, OutcomeError)
	}
	if rec2.ExitCode != -1 {
		t.Errorf("ExitCode = %d, want -1", rec2.ExitCode)
	}
	if rec2.Stderr != "boom" {
		t.Errorf("Stderr = %q, want %q", rec2.Stderr, "boom")
	}
}

// TestWrapRunnerDerivesRecorderPerCall is the MCP-fix regression test: one
// long-lived wrapped Runner instance (built once, as MCP's HandlerContext
// holds it for the server's whole lifetime) must record each call according
// to that call's OWN ctx, not a Recorder captured once when the Runner was
// constructed. A call whose ctx carries no Recorder must not panic and must
// not write to any prior call's sink.
func TestWrapRunnerDerivesRecorderPerCall(t *testing.T) {
	stub := &stubRunner{}
	wrapped := WrapRunner(stub) // constructed once, like MCP's HandlerContext.Runner

	sinkA := &fakeSink{}
	recA := Maybe(true, sinkA, "add", "", "", "v1")
	if _, err := wrapped.RunInDir(WithRecorder(context.Background(), recA), "/tmp/a", "status"); err != nil {
		t.Fatalf("call A: %v", err)
	}

	sinkB := &fakeSink{}
	recB := Maybe(true, sinkB, "remove", "", "", "v1")
	if _, err := wrapped.RunInDir(WithRecorder(context.Background(), recB), "/tmp/b", "status"); err != nil {
		t.Fatalf("call B: %v", err)
	}

	if _, err := wrapped.RunInDir(context.Background(), "/tmp/c", "status"); err != nil {
		t.Fatalf("call C (no recorder): %v", err)
	}

	if len(sinkA.logs) != 1 {
		t.Errorf("sinkA.logs = %d, want 1 (call A's own record only)", len(sinkA.logs))
	}
	if len(sinkB.logs) != 1 {
		t.Errorf("sinkB.logs = %d, want 1 (call B's own record only)", len(sinkB.logs))
	}
}

func TestWrapRunnerNoRecorderFallsBackToDebugTiming(t *testing.T) {
	t.Setenv("RIMBA_DEBUG", "1")
	stub := &stubRunner{}
	wrapped := WrapRunner(stub)

	r, w, _ := os.Pipe()
	origStderr := os.Stderr
	os.Stderr = w
	t.Cleanup(func() { os.Stderr = origStderr })

	if _, err := wrapped.RunInDir(context.Background(), "/tmp/worktree", "fetch"); err != nil {
		t.Fatalf("RunInDir: %v", err)
	}

	w.Close()
	output, _ := io.ReadAll(r)
	if !strings.Contains(string(output), "[debug] git fetch [worktree]") {
		t.Errorf("expected RIMBA_DEBUG fallback line, got %q", output)
	}
}

func TestWrapRunnerNoRecorderNoDebugIsSilent(t *testing.T) {
	os.Unsetenv("RIMBA_DEBUG")
	t.Cleanup(func() { os.Unsetenv("RIMBA_DEBUG") })
	stub := &stubRunner{}
	wrapped := WrapRunner(stub)

	r, w, _ := os.Pipe()
	origStderr := os.Stderr
	os.Stderr = w
	t.Cleanup(func() { os.Stderr = origStderr })

	if _, err := wrapped.Run(context.Background(), "status"); err != nil {
		t.Fatalf("Run: %v", err)
	}

	w.Close()
	output, _ := io.ReadAll(r)
	if len(output) != 0 {
		t.Errorf("expected silence with no recorder and no RIMBA_DEBUG, got %q", output)
	}
}

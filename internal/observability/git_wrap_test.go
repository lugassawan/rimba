package observability

import (
	"context"
	"errors"
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

func TestWrapRunnerNilRecorderReturnsSameRunner(t *testing.T) {
	stub := &stubRunner{}

	wrapped := WrapRunner(stub, nil)

	if wrapped != stub {
		t.Error("WrapRunner(inner, nil) should return inner unchanged")
	}
}

func TestWrapRunnerRunRecordsSuccess(t *testing.T) {
	sink := &fakeSink{}
	rec := Maybe(true, sink, "add", "task", "svc", "v1")
	stub := &stubRunner{}

	wrapped := WrapRunner(stub, rec)
	out, err := wrapped.Run(context.Background(), "status", "-s")
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

	wrapped := WrapRunner(stub, rec)
	_, err := wrapped.RunInDir(context.Background(), "/tmp/worktree", "fetch")
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

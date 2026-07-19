package executor

import (
	"context"
	"errors"
	"testing"

	"github.com/lugassawan/rimba/internal/observability"
)

// fakeSink is a tiny in-memory test double for observability.Sink, mirroring
// the one in internal/observability/recorder_test.go.
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

func TestWrapRunFuncNilRecorderBehavesLikeInner(t *testing.T) {
	fn := mockRunner("out", "err", 0, nil)

	wrapped := WrapRunFunc(fn, nil)

	wantStdout, wantStderr, wantExit, wantErr := fn(context.Background(), "/tmp", "echo hi")
	gotStdout, gotStderr, gotExit, gotErr := wrapped(context.Background(), "/tmp", "echo hi")

	if string(gotStdout) != string(wantStdout) || string(gotStderr) != string(wantStderr) ||
		gotExit != wantExit || !errors.Is(gotErr, wantErr) {
		t.Errorf("WrapRunFunc(fn, nil) behaved differently: got (%q,%q,%d,%v), want (%q,%q,%d,%v)",
			gotStdout, gotStderr, gotExit, gotErr, wantStdout, wantStderr, wantExit, wantErr)
	}
}

func TestWrapRunFuncRecordsSuccess(t *testing.T) {
	sink := &fakeSink{}
	rec := observability.Maybe(true, sink, "exec", "task", "svc", "v1")
	fn := mockRunner("hello\n", "", 0, nil)

	wrapped := WrapRunFunc(fn, rec)
	_, _, _, err := wrapped(context.Background(), "/tmp/worktree", "echo hello")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(sink.logs) != 1 {
		t.Fatalf("len(sink.logs) = %d, want 1", len(sink.logs))
	}
	subRec, ok := sink.logs[0].(observability.SubprocessRecord)
	if !ok {
		t.Fatalf("sink.logs[0] = %T, want SubprocessRecord", sink.logs[0])
	}
	if subRec.Category != observability.CategoryExec {
		t.Errorf("Category = %q, want %q", subRec.Category, observability.CategoryExec)
	}
	if subRec.Outcome != observability.OutcomeSuccess {
		t.Errorf("Outcome = %q, want %q", subRec.Outcome, observability.OutcomeSuccess)
	}
	if subRec.Stderr != "" {
		t.Errorf("Stderr = %q, want empty", subRec.Stderr)
	}
	if len(subRec.Args) != 1 || subRec.Args[0] != "echo hello" {
		t.Errorf("Args = %v, want [echo hello]", subRec.Args)
	}
	if subRec.Dir != "/tmp/worktree" {
		t.Errorf("Dir = %q, want %q", subRec.Dir, "/tmp/worktree")
	}
}

func TestWrapRunFuncRecordsExitCodeFailureWithoutErr(t *testing.T) {
	sink := &fakeSink{}
	rec := observability.Maybe(true, sink, "exec", "task", "svc", "v1")
	fn := mockRunner("", "boom", 1, nil)

	wrapped := WrapRunFunc(fn, rec)
	_, _, exitCode, err := wrapped(context.Background(), "/tmp/worktree", "false")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if exitCode != 1 {
		t.Fatalf("exitCode = %d, want 1", exitCode)
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
	if subRec.Stderr != "boom" {
		t.Errorf("Stderr = %q, want %q", subRec.Stderr, "boom")
	}
}

func TestWrapRunFuncRecordsStartFailure(t *testing.T) {
	sink := &fakeSink{}
	rec := observability.Maybe(true, sink, "exec", "task", "svc", "v1")
	fn := mockRunner("", "", 0, errors.New("no such file"))

	wrapped := WrapRunFunc(fn, rec)
	_, _, _, err := wrapped(context.Background(), "/tmp/worktree", "nonexistent")
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
}

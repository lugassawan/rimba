package cmd

import (
	"context"
	"os"
	"testing"

	"github.com/lugassawan/rimba/internal/debug"
	"github.com/lugassawan/rimba/internal/git"
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

func TestNewRunnerNoRecorderNoDebugReturnsBaseRunner(t *testing.T) {
	os.Unsetenv("RIMBA_DEBUG")
	t.Cleanup(func() { os.Unsetenv("RIMBA_DEBUG") })

	r := newRunner(context.Background())

	if _, ok := r.(*git.ExecRunner); !ok {
		t.Errorf("newRunner() = %T, want *git.ExecRunner (no recorder, no RIMBA_DEBUG)", r)
	}
}

func TestNewRunnerNoRecorderWithDebugFallsBackToDebugWrapper(t *testing.T) {
	t.Setenv("RIMBA_DEBUG", "1")

	r := newRunner(context.Background())

	if _, ok := r.(*debug.TimedRunner); !ok {
		t.Errorf("newRunner() = %T, want *debug.TimedRunner (no recorder, RIMBA_DEBUG set)", r)
	}
}

func TestNewRunnerWithRecorderRecordsGitCalls(t *testing.T) {
	os.Unsetenv("RIMBA_DEBUG")
	t.Cleanup(func() { os.Unsetenv("RIMBA_DEBUG") })

	sink := &fakeSink{}
	rec := observability.Maybe(true, sink, "status", "", "", "v1")
	ctx := observability.WithRecorder(context.Background(), rec)

	r := newRunner(ctx)
	if _, err := r.Run(ctx, "--version"); err != nil {
		t.Fatalf("git --version failed: %v", err)
	}

	if len(sink.logs) != 1 {
		t.Fatalf("len(sink.logs) = %d, want 1", len(sink.logs))
	}
	subRec, ok := sink.logs[0].(observability.SubprocessRecord)
	if !ok {
		t.Fatalf("sink.logs[0] = %T, want SubprocessRecord", sink.logs[0])
	}
	if subRec.Category != observability.CategoryGit {
		t.Errorf("Category = %q, want %q", subRec.Category, observability.CategoryGit)
	}
}

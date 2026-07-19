package cmd

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
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

// TestNewRunnerAlwaysWrapsWithObservability confirms newRunner unconditionally
// returns the observability-wrapped runner regardless of whether ctx carries
// a Recorder at construction time — the decorator itself derives the
// Recorder fresh per call (see observability.WrapRunner), which is what lets
// a single instance built once (as MCP's HandlerContext holds it) still
// record each tool call correctly.
func TestNewRunnerAlwaysWrapsWithObservability(t *testing.T) {
	os.Unsetenv("RIMBA_DEBUG")
	t.Cleanup(func() { os.Unsetenv("RIMBA_DEBUG") })

	r := newRunner(context.Background())

	if _, err := r.Run(context.Background(), "--version"); err != nil {
		t.Fatalf("git --version failed: %v", err)
	}
}

func TestNewRunnerNoRecorderWithDebugFallsBackToDebugTiming(t *testing.T) {
	t.Setenv("RIMBA_DEBUG", "1")

	r, w, _ := os.Pipe()
	origStderr := os.Stderr
	os.Stderr = w
	t.Cleanup(func() { os.Stderr = origStderr })

	rn := newRunner(context.Background())
	if _, err := rn.Run(context.Background(), "--version"); err != nil {
		t.Fatalf("git --version failed: %v", err)
	}

	w.Close()
	output, _ := io.ReadAll(r)
	if !strings.Contains(string(output), "[debug] git --version") {
		t.Errorf("expected RIMBA_DEBUG fallback line, got %q", output)
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

// TestNewRunnerDerivesRecorderPerCall is the MCP-fix regression test at the
// composition-root level: one newRunner-built instance (as MCP's
// HandlerContext.Runner holds for the server's lifetime) must record each
// call according to that call's own ctx, not one fixed at construction.
func TestNewRunnerDerivesRecorderPerCall(t *testing.T) {
	os.Unsetenv("RIMBA_DEBUG")
	t.Cleanup(func() { os.Unsetenv("RIMBA_DEBUG") })

	// Built once with no Recorder on ctx — mirrors cmd/mcp.go's startup call.
	r := newRunner(context.Background())

	sink := &fakeSink{}
	rec := observability.Maybe(true, sink, "add", "", "", "v1")
	callCtx := observability.WithRecorder(context.Background(), rec)

	if _, err := r.Run(callCtx, "--version"); err != nil {
		t.Fatalf("git --version failed: %v", err)
	}

	if len(sink.logs) != 1 {
		t.Fatalf("len(sink.logs) = %d, want 1 (per-call recorder must still be used)", len(sink.logs))
	}
}

// withFakeGhOnPath installs a `gh` script on PATH for the test's lifetime,
// so newGHRunner's underlying gh.Default execRunner has something to shell
// out to. PATH is prepended (not replaced) so other tools stay resolvable.
func withFakeGhOnPath(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	fake := filepath.Join(dir, "gh")
	if err := os.WriteFile(fake, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func TestNewGHRunnerRecordsCallsPerCallCtx(t *testing.T) {
	withFakeGhOnPath(t)

	sink := &fakeSink{}
	rec := observability.Maybe(true, sink, "add", "", "", "v1")
	ctx := observability.WithRecorder(context.Background(), rec)

	r := newGHRunner(ctx)
	if _, err := r.Run(ctx, "pr", "view"); err != nil {
		t.Fatalf("gh pr view failed: %v", err)
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
}

// TestNewGHRunnerDerivesRecorderPerCall is the MCP-fix regression test at the
// composition-root level for gh: one newGHRunner-built instance (as MCP's
// HandlerContext.GH holds for the server's lifetime) must record each call
// according to that call's own ctx, not one fixed at construction.
func TestNewGHRunnerDerivesRecorderPerCall(t *testing.T) {
	withFakeGhOnPath(t)

	// Built once with no Recorder on ctx — mirrors cmd/mcp.go's startup call.
	r := newGHRunner(context.Background())

	sink := &fakeSink{}
	rec := observability.Maybe(true, sink, "add", "", "", "v1")
	callCtx := observability.WithRecorder(context.Background(), rec)

	if _, err := r.Run(callCtx, "pr", "view"); err != nil {
		t.Fatalf("gh pr view failed: %v", err)
	}

	if len(sink.logs) != 1 {
		t.Fatalf("len(sink.logs) = %d, want 1 (per-call recorder must still be used)", len(sink.logs))
	}
}

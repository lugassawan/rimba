package cmd

import (
	"strings"
	"testing"

	"github.com/lugassawan/rimba/internal/executor"
	"github.com/lugassawan/rimba/internal/resolver"
	"github.com/lugassawan/rimba/internal/spinner"
	"github.com/lugassawan/rimba/internal/termcolor"
	"github.com/spf13/cobra"
)

func TestHasFailureNonZeroExit(t *testing.T) {
	results := []executor.Result{
		{ExitCode: 0},
		{ExitCode: 1},
	}
	if !hasFailure(results) {
		t.Error("expected hasFailure=true for non-zero exit")
	}
}

func TestHasFailureWithError(t *testing.T) {
	results := []executor.Result{
		{Err: errGitFailed},
	}
	if !hasFailure(results) {
		t.Error("expected hasFailure=true for error")
	}
}

func TestHasFailureAllSuccess(t *testing.T) {
	results := []executor.Result{
		{ExitCode: 0},
		{ExitCode: 0},
	}
	if hasFailure(results) {
		t.Error("expected hasFailure=false for all success")
	}
}

func TestHasFailureCancelledOnly(t *testing.T) {
	results := []executor.Result{
		{Cancelled: true},
	}
	if hasFailure(results) {
		t.Error("expected hasFailure=false for cancelled-only (trigger result has non-zero exit)")
	}
}

func TestHasFailureEmpty(t *testing.T) {
	if hasFailure(nil) {
		t.Error("expected hasFailure=false for empty results")
	}
}

func TestPrintExecResultsOk(t *testing.T) {
	cmd, buf := newTestCmd()
	p := termcolor.NewPainter(true)

	results := []executor.Result{
		{
			Target: executor.Target{Branch: "feature/login", Task: "login"},
			Stdout: []byte("hello\n"),
		},
	}

	printExecResults(cmd, p, results, resolver.AllPrefixes())
	out := buf.String()

	if !strings.Contains(out, "login") {
		t.Errorf("expected task name in output, got: %s", out)
	}
	if !strings.Contains(out, "ok") {
		t.Errorf("expected 'ok' status in output, got: %s", out)
	}
	if !strings.Contains(out, "hello") {
		t.Errorf("expected stdout in output, got: %s", out)
	}
}

func TestPrintExecResultsNonZeroExit(t *testing.T) {
	cmd, buf := newTestCmd()
	p := termcolor.NewPainter(true)

	results := []executor.Result{
		{
			Target:   executor.Target{Branch: "feature/x", Task: "x"},
			ExitCode: 1,
			Stderr:   []byte("fail\n"),
		},
	}

	printExecResults(cmd, p, results, resolver.AllPrefixes())
	out := buf.String()

	if !strings.Contains(out, "exit 1") {
		t.Errorf("expected 'exit 1' in output, got: %s", out)
	}
	if !strings.Contains(out, "fail") {
		t.Errorf("expected stderr in output, got: %s", out)
	}
}

func TestPrintExecResultsCancelled(t *testing.T) {
	cmd, buf := newTestCmd()
	p := termcolor.NewPainter(true)

	results := []executor.Result{
		{
			Target:    executor.Target{Branch: "feature/y", Task: "y"},
			Cancelled: true,
		},
	}

	printExecResults(cmd, p, results, resolver.AllPrefixes())
	out := buf.String()

	if !strings.Contains(out, "cancelled") {
		t.Errorf("expected 'cancelled' in output, got: %s", out)
	}
}

func TestPrintExecResultsError(t *testing.T) {
	cmd, buf := newTestCmd()
	p := termcolor.NewPainter(true)

	results := []executor.Result{
		{
			Target: executor.Target{Branch: "feature/z", Task: "z"},
			Err:    errGitFailed,
		},
	}

	printExecResults(cmd, p, results, resolver.AllPrefixes())
	out := buf.String()

	if !strings.Contains(out, "error") {
		t.Errorf("expected 'error' status in output, got: %s", out)
	}
	if !strings.Contains(out, "git failed") {
		t.Errorf("expected error message in output, got: %s", out)
	}
}

func TestFilterDirtyWorktrees(t *testing.T) {
	r := &mockRunner{
		run: func(_ ...string) (string, error) { return "", nil },
		runInDir: func(dir string, _ ...string) (string, error) {
			if dir == "/dirty" {
				return "M file.go", nil
			}
			return "", nil
		},
	}

	worktrees := []resolver.WorktreeInfo{
		{Path: "/clean", Branch: "feature/a"},
		{Path: "/dirty", Branch: "feature/b"},
		{Path: "/also-clean", Branch: "feature/c"},
	}

	cmd, _ := newTestCmd()
	s := testExecSpinner(cmd)
	defer s.Stop()

	result := filterDirtyWorktrees(r, s, worktrees)
	if len(result) != 1 {
		t.Fatalf("expected 1 dirty worktree, got %d", len(result))
	}
	if result[0].Path != "/dirty" {
		t.Errorf("expected /dirty, got %s", result[0].Path)
	}
}

func testExecSpinner(cmd *cobra.Command) *spinner.Spinner {
	return spinner.New(spinnerOpts(cmd))
}

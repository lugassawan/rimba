package cmd

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/lugassawan/rimba/internal/executor"
	"github.com/lugassawan/rimba/internal/output"
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

func TestPrintExecResultsJSON(t *testing.T) {
	cmd, buf := newTestCmd()
	_ = cmd.Flags().Set(flagJSON, "true")

	results := []executor.Result{
		{
			Target:   executor.Target{Branch: "feature/login", Task: "login", Path: "/wt/login"},
			Stdout:   []byte("hello\n"),
			ExitCode: 0,
		},
		{
			Target:   executor.Target{Branch: "feature/fail", Task: "fail", Path: "/wt/fail"},
			ExitCode: 1,
			Stderr:   []byte("error\n"),
		},
	}

	// Simulate the JSON output path from exec command
	jsonResults := make([]output.ExecResult, len(results))
	for i, r := range results {
		jr := output.ExecResult{
			Task:     r.Target.Task,
			Branch:   r.Target.Branch,
			Path:     r.Target.Path,
			ExitCode: r.ExitCode,
			Stdout:   string(r.Stdout),
			Stderr:   string(r.Stderr),
		}
		if r.Err != nil {
			jr.Error = r.Err.Error()
		}
		jsonResults[i] = jr
	}
	data := output.ExecData{
		Command: "echo hello",
		Results: jsonResults,
		Success: !hasFailure(results),
	}
	_ = output.WriteJSON(cmd.OutOrStdout(), version, "exec", data)

	var env output.Envelope
	if err := json.Unmarshal(buf.Bytes(), &env); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if env.Command != "exec" {
		t.Errorf("command = %q, want %q", env.Command, "exec")
	}
	dataMap, ok := env.Data.(map[string]any)
	if !ok {
		t.Fatalf("data type = %T, want map[string]any", env.Data)
	}
	if dataMap["success"] != false {
		t.Error("expected success=false when one command fails")
	}
	resultsArr, ok := dataMap["results"].([]any)
	if !ok {
		t.Fatalf("results type = %T, want []any", dataMap["results"])
	}
	if len(resultsArr) != 2 {
		t.Errorf("results length = %d, want 2", len(resultsArr))
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

func TestExecTypeFlagCompletion(t *testing.T) {
	fn, ok := execCmd.GetFlagCompletionFunc(flagType)
	if !ok {
		t.Fatal("no completion function registered for --type flag")
	}

	t.Run("all types", func(t *testing.T) {
		types, directive := fn(execCmd, nil, "")
		if directive != cobra.ShellCompDirectiveNoFileComp {
			t.Errorf(directiveWantFmt, directive)
		}
		if len(types) == 0 {
			t.Fatal("expected at least one type completion")
		}
	})

	t.Run("filter by prefix", func(t *testing.T) {
		types, _ := fn(execCmd, nil, "bug")
		if len(types) != 1 {
			t.Fatalf("expected 1 type, got %d: %v", len(types), types)
		}
		if types[0] != benchFilterType {
			t.Errorf("type = %q, want %q", types[0], benchFilterType)
		}
	})

	t.Run("no match", func(t *testing.T) {
		types, _ := fn(execCmd, nil, "zzz")
		if len(types) != 0 {
			t.Errorf("expected 0 types for 'zzz', got %d: %v", len(types), types)
		}
	})
}

func testExecSpinner(cmd *cobra.Command) *spinner.Spinner {
	return spinner.New(spinnerOpts(cmd))
}

package cmd

import (
	"context"
	"strings"
	"testing"

	"github.com/lugassawan/rimba/internal/executor"
	"github.com/lugassawan/rimba/internal/observability"
)

// TestExecCmdRecorderWiresIntoRunner verifies that when a Recorder is attached
// to the command's context, the executor.Config.Runner built at runExec's
// composition root is the observability-wrapped ShellRunner: invoking the
// captured Runner produces a SubprocessRecord in the sink.
func TestExecCmdRecorderWiresIntoRunner(t *testing.T) {
	porcelain := strings.Join([]string{
		"worktree /repo",
		"HEAD abc",
		"branch refs/heads/main",
		"",
		"worktree /wt/foo",
		"HEAD def",
		"branch refs/heads/feature/foo",
		"",
	}, "\n")
	r := &mockRunner{
		run:      func(_ ...string) (string, error) { return porcelain, nil },
		runInDir: noopRunInDir,
	}

	var capturedCfg executor.Config
	fakeExec := func(_ context.Context, cfg executor.Config) []executor.Result {
		capturedCfg = cfg
		return []executor.Result{
			{Target: executor.Target{Branch: "feature/foo", Task: "foo", Path: "/wt/foo"}, ExitCode: 0},
		}
	}

	cmd, _ := newExecCmd(r, fakeExec)
	sink := &fakeSink{}
	rec := observability.Maybe(true, sink, "exec", "", "", "v1")
	cmd.SetContext(observability.WithRecorder(cmd.Context(), rec))
	cmd.SetArgs([]string{"--all", "--no-color", "echo ok"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if capturedCfg.Runner == nil {
		t.Fatal("expected a non-nil Runner on the captured executor.Config")
	}
	if _, _, _, err := capturedCfg.Runner(context.Background(), t.TempDir(), "echo ok"); err != nil {
		t.Fatalf("Runner invocation failed: %v", err)
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
}

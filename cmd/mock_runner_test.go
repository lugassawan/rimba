package cmd

import (
	"bytes"
	"errors"

	"github.com/lugassawan/rimba/internal/git"
	"github.com/spf13/cobra"
)

const (
	cmdBranch       = "branch"
	cmdWorktreeTest = "worktree"
	branchMain      = "main"
	branchFeature   = "feature/login"
	branchDone      = "feature/done"
	pathWtDone      = "/wt/feature-done"
	pathWorktree    = "/worktree"
	errExpected     = "expected error"
)

var errGitFailed = errors.New("git failed")

// mockRunner implements git.Runner with configurable closures for testing.
type mockRunner struct {
	run      func(args ...string) (string, error)
	runInDir func(dir string, args ...string) (string, error)
}

func (m *mockRunner) Run(args ...string) (string, error) {
	return m.run(args...)
}

func (m *mockRunner) RunInDir(dir string, args ...string) (string, error) {
	return m.runInDir(dir, args...)
}

// noopRunInDir is a default runInDir that returns empty output.
func noopRunInDir(_ string, _ ...string) (string, error) {
	return "", nil
}

// newTestCmd creates a cobra.Command with --no-color flag and a bytes.Buffer for output capture.
func newTestCmd() (*cobra.Command, *bytes.Buffer) {
	buf := new(bytes.Buffer)
	cmd := &cobra.Command{Use: "test"}
	cmd.Flags().Bool(flagNoColor, true, "")
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	return cmd, buf
}

// overrideNewRunner temporarily replaces the newRunner function for testing.
func overrideNewRunner(r git.Runner) func() {
	orig := newRunner
	newRunner = func() git.Runner { return r }
	return func() { newRunner = orig }
}

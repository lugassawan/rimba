package cmd

import (
	"bytes"
	"context"
	"errors"

	"github.com/lugassawan/rimba/internal/gh"
	"github.com/lugassawan/rimba/internal/git"
	"github.com/spf13/cobra"
)

const (
	cmdBranch       = "branch"
	cmdLog          = "log"
	cmdWorktreeTest = "worktree"
	branchMain      = "main"
	branchFeature   = "feature/login"
	branchDone      = "feature/done"
	pathWtDone      = "/wt/feature-done"
	pathWorktree    = "/worktree"
	errExpected     = "expected error"
	typeFeature     = "feature"

	// Shared porcelain helpers
	wtPrefix              = "worktree "
	headMainBlock         = "\nHEAD abc\nbranch refs/heads/main\n"
	cmdShowToplevel       = "--show-toplevel"
	cmdGitCommonDir       = "--git-common-dir"
	cmdRevParse           = "rev-parse"
	dirtyOutput           = "M dirty.go"
	headABC123            = "HEAD abc123"
	branchRefMain         = "branch refs/heads/main"
	headDEF456            = "HEAD def456"
	branchRefFeatureLogin = "branch refs/heads/feature/login"
	wtRepo                = "worktree /repo"
	defaultRelativeWtDir  = "../worktrees"
	pathWtFeatureLogin    = "/wt/feature-login"
	branchBugfixTypo      = "bugfix/typo"
	msgRemovedWorktree    = "Removed worktree"

	branchRefPrefix           = "branch refs/heads/"
	wtDone                    = "worktree " + pathWtDone
	wtFeatureLogin            = "worktree /wt/feature-login"
	pathWorktreesFeatureLogin = "/worktrees/feature-login"
	cmdRevList                = "rev-list"

	// Shared git command and value constants
	cmdList   = "list"
	cmdDiff   = "diff"
	cmdFetch  = "fetch"
	cmdPrune  = "prune"
	cmdRemote = "remote"
	cmdRemove = "remove"

	twoRemotesList        = "origin\nupstream\n"
	gitSubcmdWorktreeAdd  = "add"
	cmdSymbolicRef        = "symbolic-ref"
	cmdStatus             = "status"
	refsRemotesOriginMain = "refs/remotes/origin/main"
	aheadBehindZero       = "0\t0"
	repoPath              = "/repo"

	// orphanedProjWorktreeOut lists a main worktree plus one under "PROJ-",
	// shared across orphan-guard tests where "PROJ-" isn't configured.
	orphanedProjWorktreeOut = "worktree /repo\nHEAD abc123\nbranch refs/heads/main\n\nworktree /wt/proj-123\nHEAD def456\nbranch refs/heads/PROJ-123\n"

	branchDevelop    = "develop"
	taskLogin        = "login"
	taskWantFmt      = "task = %q, want %q"
	directiveWantFmt = "directive = %v, want ShellCompDirectiveNoFileComp"
	filterByPrefix   = "filter by prefix"
	useTestCmd       = "test-cmd"

	// Shared branch list outputs
	branchListArchived = "main\nfeature/archived-task\nfeature/active-task"

	// Shared fatalf format strings
	fatalMergeRunE   = "mergeCmd.RunE: %v"
	fatalCleanPrune  = "cleanPrune: %v"
	fatalCleanMerged = "cleanMerged: %v"
	fatalListRunE    = "listCmd.RunE: %v"
	fatalSyncAll     = "syncAll: %v"
)

var errGitFailed = errors.New("git failed")

// mockRunner implements git.Runner with configurable closures for testing.
type mockRunner struct {
	run      func(args ...string) (string, error)
	runInDir func(dir string, args ...string) (string, error)
}

func (m *mockRunner) Run(_ context.Context, args ...string) (string, error) {
	return m.run(args...)
}

func (m *mockRunner) RunInDir(_ context.Context, dir string, args ...string) (string, error) {
	return m.runInDir(dir, args...)
}

// noopRunInDir is a default runInDir that returns empty output.
func noopRunInDir(_ string, _ ...string) (string, error) {
	return "", nil
}

// newTestCmd creates a cobra.Command with --no-color and --json flags
// and a bytes.Buffer for output capture.
func newTestCmd() (*cobra.Command, *bytes.Buffer) {
	buf := new(bytes.Buffer)
	cmd := &cobra.Command{Use: "test"}
	cmd.Flags().Bool(flagNoColor, true, "")
	cmd.Flags().Bool(flagJSON, false, "")
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetContext(context.Background())
	return cmd, buf
}

// overrideNewRunner temporarily replaces the newRunner function for testing.
func overrideNewRunner(r git.Runner) func() {
	orig := newRunner
	newRunner = func(_ context.Context) git.Runner { return r }
	return func() { newRunner = orig }
}

// mockGhRunner implements gh.Runner with a configurable closure for testing.
type mockGhRunner struct {
	run func(ctx context.Context, args ...string) ([]byte, error)
}

func (m *mockGhRunner) Run(ctx context.Context, args ...string) ([]byte, error) {
	return m.run(ctx, args...)
}

// overrideGHRunner temporarily replaces the newGHRunner function for testing.
func overrideGHRunner(r gh.Runner) func() {
	orig := newGHRunner
	newGHRunner = func(_ context.Context) gh.Runner { return r }
	return func() { newGHRunner = orig }
}

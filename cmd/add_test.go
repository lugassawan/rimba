package cmd

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lugassawan/rimba/internal/config"
	"github.com/lugassawan/rimba/testutil"
	"github.com/spf13/cobra"
)

const (
	prCmdAuth = "auth"
	prCmdPR   = "pr"

	// branch: promote mode constants
	branchFeatureX     = "feature/x"
	stashSHACmd        = "deadbeef123"
	cmdStash           = "stash"
	cmdSwitch          = "switch"
	gitSubcmdStashPush = "push"
	gitSubcmdStashAppl = "apply"
	gitSubcmdStashDrop = "drop"
	cmdFlagVerify      = "--verify"
	cmdFlagShort       = "--short"
	stashListLineCmd   = stashSHACmd + " stash@{0}"
)

// makeWorktreeGitRunner builds a mockRunner that simulates a successful worktree creation.
// Handles: git common-dir, show-toplevel, fetch, rev-parse (branch absent), worktree add.
func makeWorktreeGitRunner(repoDir string) *mockRunner {
	return &mockRunner{
		run: func(args ...string) (string, error) {
			switch {
			case len(args) >= 2 && args[1] == cmdGitCommonDir:
				return filepath.Join(repoDir, ".git"), nil
			case len(args) >= 2 && args[1] == cmdShowToplevel:
				return repoDir, nil
			case len(args) >= 1 && args[0] == cmdFetch:
				return "", nil
			case len(args) >= 1 && args[0] == cmdRevParse:
				return "", errGitFailed
			case len(args) >= 1 && args[0] == cmdWorktreeTest:
				if len(args) >= 2 && args[1] == gitSubcmdWorktreeAdd {
					_ = os.MkdirAll(args[4], 0o755)
				}
				return "", nil
			}
			return "", nil
		},
		runInDir: noopRunInDir,
	}
}

// makeOKGhRunner builds a mockGhRunner that returns a successful auth and the given PR JSON.
func makeOKGhRunner(prJSON string) *mockGhRunner {
	return &mockGhRunner{
		run: func(_ context.Context, args ...string) ([]byte, error) {
			if len(args) > 0 && args[0] == prCmdAuth {
				return []byte("Logged in"), nil
			}
			return []byte(prJSON), nil
		},
	}
}

func TestAddSuccess(t *testing.T) {
	repoDir := t.TempDir()
	wtDir := filepath.Join(repoDir, "worktrees")
	_ = os.MkdirAll(wtDir, 0755)
	cfg := &config.Config{DefaultSource: branchMain, WorktreeDir: "worktrees"}

	worktreeOut := wtPrefix + repoDir + headMainBlock

	r := &mockRunner{
		run: func(args ...string) (string, error) {
			if len(args) >= 2 && args[1] == cmdGitCommonDir {
				return filepath.Join(repoDir, ".git"), nil
			}
			if len(args) >= 2 && args[1] == cmdShowToplevel {
				return repoDir, nil
			}
			if len(args) >= 1 && args[0] == cmdRevParse {
				return "", errGitFailed // BranchExists returns false
			}
			return worktreeOut, nil
		},
		runInDir: noopRunInDir,
	}
	restore := overrideNewRunner(r)
	defer restore()

	cmd, buf := newTestCmd()
	cmd.Flags().StringP(flagSource, "s", "", "")
	cmd.Flags().Bool(flagSkipDeps, false, "")
	cmd.Flags().Bool(flagSkipHooks, false, "")
	addPrefixFlags(cmd)
	_ = cmd.Flags().Set(flagSkipDeps, "true")
	_ = cmd.Flags().Set(flagSkipHooks, "true")
	cmd.SetContext(config.WithConfig(context.Background(), cfg))

	err := addCmd.RunE(cmd, []string{"my-task"})
	if err != nil {
		t.Fatalf("addCmd.RunE: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "Created worktree") {
		t.Errorf("output = %q, want 'Created worktree'", out)
	}
	if !strings.Contains(out, "my-task") {
		t.Errorf("output = %q, want 'my-task'", out)
	}
}

func TestAddBranchAlreadyExists(t *testing.T) {
	repoDir := t.TempDir()
	cfg := &config.Config{DefaultSource: branchMain, WorktreeDir: "worktrees"}

	worktreeOut := wtPrefix + repoDir + headMainBlock

	r := &mockRunner{
		run: func(args ...string) (string, error) {
			if len(args) >= 2 && args[1] == cmdGitCommonDir {
				return filepath.Join(repoDir, ".git"), nil
			}
			if len(args) >= 2 && args[1] == cmdShowToplevel {
				return repoDir, nil
			}
			if len(args) >= 1 && args[0] == cmdRevParse {
				return "", nil // BranchExists returns true
			}
			return worktreeOut, nil
		},
		runInDir: noopRunInDir,
	}
	restore := overrideNewRunner(r)
	defer restore()

	cmd, _ := newTestCmd()
	cmd.Flags().StringP(flagSource, "s", "", "")
	cmd.Flags().Bool(flagSkipDeps, false, "")
	cmd.Flags().Bool(flagSkipHooks, false, "")
	addPrefixFlags(cmd)
	cmd.SetContext(config.WithConfig(context.Background(), cfg))

	err := addCmd.RunE(cmd, []string{"existing"})
	if err == nil {
		t.Fatal("expected error for existing branch")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Errorf("error = %q, want 'already exists'", err.Error())
	}
}

func TestAddWithSource(t *testing.T) {
	repoDir := t.TempDir()
	wtDir := filepath.Join(repoDir, "worktrees")
	_ = os.MkdirAll(wtDir, 0755)
	cfg := &config.Config{DefaultSource: branchMain, WorktreeDir: "worktrees"}

	worktreeOut := wtPrefix + repoDir + headMainBlock

	r := &mockRunner{
		run: func(args ...string) (string, error) {
			if len(args) >= 2 && args[1] == cmdGitCommonDir {
				return filepath.Join(repoDir, ".git"), nil
			}
			if len(args) >= 2 && args[1] == cmdShowToplevel {
				return repoDir, nil
			}
			if len(args) >= 1 && args[0] == cmdRevParse {
				return "", errGitFailed
			}
			return worktreeOut, nil
		},
		runInDir: noopRunInDir,
	}
	restore := overrideNewRunner(r)
	defer restore()

	cmd, buf := newTestCmd()
	cmd.Flags().StringP(flagSource, "s", "", "")
	cmd.Flags().Bool(flagSkipDeps, false, "")
	cmd.Flags().Bool(flagSkipHooks, false, "")
	addPrefixFlags(cmd)
	_ = cmd.Flags().Set(flagSource, "develop")
	_ = cmd.Flags().Set(flagSkipDeps, "true")
	_ = cmd.Flags().Set(flagSkipHooks, "true")
	cmd.SetContext(config.WithConfig(context.Background(), cfg))

	err := addCmd.RunE(cmd, []string{"from-develop"})
	if err != nil {
		t.Fatalf("addCmd.RunE: %v", err)
	}
	if !strings.Contains(buf.String(), "Created worktree") {
		t.Errorf("output = %q, want 'Created worktree'", buf.String())
	}
}

func TestAddPRCmd(t *testing.T) {
	repoDir := t.TempDir()
	wtDir := filepath.Join(repoDir, ".worktrees")
	_ = os.MkdirAll(wtDir, 0755)
	cfg := &config.Config{
		WorktreeDir: ".worktrees",
		Deps:        &config.DepsConfig{Modules: []config.ModuleConfig{{Dir: "frontend", Install: "npm install"}}},
	}

	fakeGhDir := t.TempDir()
	fakeGh := filepath.Join(fakeGhDir, "gh")
	_ = os.WriteFile(fakeGh, []byte("#!/bin/sh\nexit 0\n"), 0o755)
	t.Setenv("PATH", fakeGhDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	sameRepoPRJSON := testutil.LoadFixture(t, "../internal/gh/testdata/same_repo_pr.json")
	restoreGH := overrideGHRunner(makeOKGhRunner(sameRepoPRJSON))
	defer restoreGH()
	restoreRunner := overrideNewRunner(makeWorktreeGitRunner(repoDir))
	defer restoreRunner()

	cmd, buf := newTestCmd()
	cmd.Flags().StringP(flagSource, "s", "", "")
	cmd.Flags().Bool(flagSkipDeps, false, "")
	cmd.Flags().Bool(flagSkipHooks, false, "")
	cmd.Flags().String(flagTask, "", "")
	addPrefixFlags(cmd)
	_ = cmd.Flags().Set(flagSkipDeps, "true")
	_ = cmd.Flags().Set(flagSkipHooks, "true")
	cmd.SetContext(config.WithConfig(context.Background(), cfg))

	err := addCmd.RunE(cmd, []string{"pr:42"})
	if err != nil {
		t.Fatalf("addCmd.RunE pr:42: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "PR #42") {
		t.Errorf("output = %q, want 'PR #42'", out)
	}
}

func TestAddPRCmdError(t *testing.T) {
	repoDir := t.TempDir()
	cfg := &config.Config{WorktreeDir: ".worktrees"}

	fakeGhDir := t.TempDir()
	fakeGh := filepath.Join(fakeGhDir, "gh")
	_ = os.WriteFile(fakeGh, []byte("#!/bin/sh\nexit 0\n"), 0o755)
	t.Setenv("PATH", fakeGhDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	ghR := &mockGhRunner{
		run: func(_ context.Context, _ ...string) ([]byte, error) {
			return nil, errors.New("gh: not authenticated")
		},
	}
	restoreGH := overrideGHRunner(ghR)
	defer restoreGH()

	r := &mockRunner{
		run: func(args ...string) (string, error) {
			if len(args) >= 2 && args[1] == cmdGitCommonDir {
				return filepath.Join(repoDir, ".git"), nil
			}
			if len(args) >= 2 && args[1] == cmdShowToplevel {
				return repoDir, nil
			}
			return "", nil
		},
		runInDir: noopRunInDir,
	}
	restoreRunner := overrideNewRunner(r)
	defer restoreRunner()

	cmd, _ := newTestCmd()
	cmd.Flags().StringP(flagSource, "s", "", "")
	cmd.Flags().Bool(flagSkipDeps, false, "")
	cmd.Flags().Bool(flagSkipHooks, false, "")
	cmd.Flags().String(flagTask, "", "")
	addPrefixFlags(cmd)
	cmd.SetContext(config.WithConfig(context.Background(), cfg))

	err := addCmd.RunE(cmd, []string{"pr:42"})
	if err == nil {
		t.Fatal("expected error from PR add with auth failure")
	}
}

func TestAddWithFeaturePrefix(t *testing.T) {
	repoDir := t.TempDir()
	_ = os.MkdirAll(filepath.Join(repoDir, ".worktrees"), 0755)
	cfg := &config.Config{WorktreeDir: ".worktrees"}

	restore := overrideNewRunner(makeWorktreeGitRunner(repoDir))
	defer restore()

	cmd, buf := newTestCmd()
	cmd.Flags().StringP(flagSource, "s", "", "")
	cmd.Flags().Bool(flagSkipDeps, false, "")
	cmd.Flags().Bool(flagSkipHooks, false, "")
	addPrefixFlags(cmd)
	_ = cmd.Flags().Set(flagSkipDeps, "true")
	_ = cmd.Flags().Set(flagSkipHooks, "true")
	cmd.SetContext(config.WithConfig(context.Background(), cfg))

	err := addCmd.RunE(cmd, []string{"feature/my-task"})
	if err != nil {
		t.Fatalf("addCmd.RunE feature/my-task: %v", err)
	}
	if !strings.Contains(buf.String(), "my-task") {
		t.Errorf("output = %q, want 'my-task'", buf.String())
	}
}

func TestAddWithService(t *testing.T) {
	repoDir := t.TempDir()
	_ = os.MkdirAll(filepath.Join(repoDir, "api"), 0o755)
	_ = os.MkdirAll(filepath.Join(repoDir, ".worktrees"), 0755)
	cfg := &config.Config{WorktreeDir: ".worktrees"}

	restore := overrideNewRunner(makeWorktreeGitRunner(repoDir))
	defer restore()

	cmd, buf := newTestCmd()
	cmd.Flags().StringP(flagSource, "s", "", "")
	cmd.Flags().Bool(flagSkipDeps, false, "")
	cmd.Flags().Bool(flagSkipHooks, false, "")
	addPrefixFlags(cmd)
	_ = cmd.Flags().Set(flagSkipDeps, "true")
	_ = cmd.Flags().Set(flagSkipHooks, "true")
	cmd.SetContext(config.WithConfig(context.Background(), cfg))

	err := addCmd.RunE(cmd, []string{"api/my-task"})
	if err != nil {
		t.Fatalf("addCmd.RunE api/my-task: %v", err)
	}
	if !strings.Contains(buf.String(), "service: api") {
		t.Errorf("output = %q, want 'service: api'", buf.String())
	}
}

// addBranchFlags registers the flags required by addCmd.RunE for branch: mode.
func addBranchFlags(cmd *cobra.Command) {
	cmd.Flags().StringP(flagSource, "s", "", "")
	cmd.Flags().Bool(flagSkipDeps, false, "")
	cmd.Flags().Bool(flagSkipHooks, false, "")
}

// branchPromoteRunFn is the Run closure for happy-path branch promotion tests.
func branchPromoteRunFn(repoDir string) func(args ...string) (string, error) {
	porcelain := wtPrefix + repoDir + headMainBlock
	return func(args ...string) (string, error) {
		switch {
		case len(args) >= 2 && args[1] == cmdGitCommonDir:
			return filepath.Join(repoDir, ".git"), nil
		case len(args) >= 1 && args[0] == cmdSymbolicRef:
			return refsRemotesOriginMain, nil
		case len(args) >= 2 && args[0] == cmdRevParse && args[1] == cmdFlagVerify:
			return "", nil
		case len(args) >= 2 && args[0] == cmdWorktreeTest && args[1] == cmdList:
			return porcelain, nil
		case len(args) >= 4 && args[0] == cmdWorktreeTest && args[1] == gitSubcmdWorktreeAdd && args[2] == "--":
			_ = os.MkdirAll(args[3], 0o755)
			return "", nil
		}
		return "", nil
	}
}

// branchRunInDirIdentity handles symbolic-ref and status matches; reports the result and whether it matched.
func branchRunInDirIdentity(dirty bool, args []string) (string, bool) {
	switch {
	case len(args) >= 2 && args[0] == cmdSymbolicRef && args[1] == cmdFlagShort:
		return branchFeatureX, true
	case len(args) >= 2 && args[0] == cmdStatus && args[1] == "--porcelain":
		if dirty {
			return dirtyOutput, true
		}
		return "", true
	}
	return "", false
}

// branchRunInDirStashA handles push / rev-parse / switch operations.
func branchRunInDirStashA(args []string) (string, bool) {
	switch {
	case len(args) >= 3 && args[0] == cmdStash && args[1] == gitSubcmdStashPush:
		return "", true
	case len(args) >= 2 && args[0] == cmdRevParse && args[1] == "stash@{0}":
		return stashSHACmd, true
	case len(args) >= 2 && args[0] == cmdSwitch:
		return "", true
	}
	return "", false
}

// branchRunInDirStashB handles stash list / apply / drop operations.
func branchRunInDirStashB(args []string) (string, bool) {
	switch {
	case len(args) >= 3 && args[0] == cmdStash && args[1] == cmdList:
		return stashListLineCmd, true
	case len(args) >= 3 && args[0] == cmdStash && args[1] == gitSubcmdStashAppl:
		return "", true
	case len(args) >= 3 && args[0] == cmdStash && args[1] == gitSubcmdStashDrop:
		return "", true
	}
	return "", false
}

// branchRunInDirStash handles happy-path stash operations; reports result and whether it matched.
func branchRunInDirStash(args []string) (string, bool) {
	if out, ok := branchRunInDirStashA(args); ok {
		return out, ok
	}
	return branchRunInDirStashB(args)
}

// branchPromoteRunInDirFn is the RunInDir closure for happy-path branch promotion tests.
func branchPromoteRunInDirFn(dirty bool) func(dir string, args ...string) (string, error) {
	return func(_ string, args ...string) (string, error) {
		if out, ok := branchRunInDirIdentity(dirty, args); ok {
			return out, nil
		}
		out, _ := branchRunInDirStash(args)
		return out, nil
	}
}

// makeBranchPromoteRunner builds a mock runner for the full PromoteBranch happy path.
func makeBranchPromoteRunner(repoDir string, dirty bool) *mockRunner {
	return &mockRunner{
		run:      branchPromoteRunFn(repoDir),
		runInDir: branchPromoteRunInDirFn(dirty),
	}
}

func TestAddBranchPromoteDirty(t *testing.T) {
	repoDir := t.TempDir()
	wtDir := filepath.Join(repoDir, "worktrees")
	_ = os.MkdirAll(wtDir, 0755)
	cfg := &config.Config{WorktreeDir: "worktrees"}
	wtPath := filepath.Join(wtDir, "feature-x")

	restore := overrideNewRunner(makeBranchPromoteRunner(repoDir, true))
	defer restore()

	cmd, buf := newTestCmd()
	addBranchFlags(cmd)
	cmd.SetContext(config.WithConfig(context.Background(), cfg))

	if err := addCmd.RunE(cmd, []string{"branch:" + branchFeatureX}); err != nil {
		t.Fatalf("addCmd.RunE: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "Promoted branch") {
		t.Errorf("output %q missing 'Promoted branch'", out)
	}
	if !strings.Contains(out, wtPath) {
		t.Errorf("output %q missing worktree path %q", out, wtPath)
	}
}

func TestAddBranchPromoteClean(t *testing.T) {
	repoDir := t.TempDir()
	_ = os.MkdirAll(filepath.Join(repoDir, "worktrees"), 0755)
	cfg := &config.Config{WorktreeDir: "worktrees"}

	restore := overrideNewRunner(makeBranchPromoteRunner(repoDir, false))
	defer restore()

	cmd, buf := newTestCmd()
	addBranchFlags(cmd)
	cmd.SetContext(config.WithConfig(context.Background(), cfg))

	if err := addCmd.RunE(cmd, []string{"branch:" + branchFeatureX}); err != nil {
		t.Fatalf("addCmd.RunE clean: %v", err)
	}
	if !strings.Contains(buf.String(), "Promoted branch") {
		t.Errorf("output %q missing 'Promoted branch'", buf.String())
	}
}

func TestAddBranchRejectsExplicitSource(t *testing.T) {
	repoDir := t.TempDir()
	cfg := &config.Config{WorktreeDir: "worktrees"}

	r := &mockRunner{
		run: func(args ...string) (string, error) {
			if len(args) >= 2 && args[1] == cmdGitCommonDir {
				return filepath.Join(repoDir, ".git"), nil
			}
			return "", nil
		},
		runInDir: noopRunInDir,
	}
	restore := overrideNewRunner(r)
	defer restore()

	cmd, _ := newTestCmd()
	addBranchFlags(cmd)
	_ = cmd.Flags().Set(flagSource, "develop")
	cmd.SetContext(config.WithConfig(context.Background(), cfg))

	err := addCmd.RunE(cmd, []string{"branch:" + branchFeatureX})
	if err == nil {
		t.Fatal("expected error for --source with branch: mode")
	}
	if !strings.Contains(err.Error(), "--source is not valid in branch: mode") {
		t.Errorf("error %q should mention --source", err)
	}
}

func TestAddBranchRejectsMain(t *testing.T) {
	repoDir := t.TempDir()
	cfg := &config.Config{WorktreeDir: "worktrees"}

	r := &mockRunner{
		run: func(args ...string) (string, error) {
			switch {
			case len(args) >= 2 && args[1] == cmdGitCommonDir:
				return filepath.Join(repoDir, ".git"), nil
			case len(args) >= 1 && args[0] == cmdSymbolicRef:
				return refsRemotesOriginMain, nil
			}
			return "", nil
		},
		runInDir: noopRunInDir,
	}
	restore := overrideNewRunner(r)
	defer restore()

	cmd, _ := newTestCmd()
	addBranchFlags(cmd)
	cmd.SetContext(config.WithConfig(context.Background(), cfg))

	err := addCmd.RunE(cmd, []string{"branch:main"})
	if err == nil {
		t.Fatal("expected error for branch:main")
	}
	if !strings.Contains(err.Error(), "cannot promote default branch") {
		t.Errorf("error %q should mention 'cannot promote default branch'", err)
	}
}

func TestAddBranchRejectsBranchMissing(t *testing.T) {
	repoDir := t.TempDir()
	cfg := &config.Config{WorktreeDir: "worktrees"}

	r := &mockRunner{
		run: func(args ...string) (string, error) {
			switch {
			case len(args) >= 2 && args[1] == cmdGitCommonDir:
				return filepath.Join(repoDir, ".git"), nil
			case len(args) >= 1 && args[0] == cmdSymbolicRef:
				return refsRemotesOriginMain, nil
			case len(args) >= 2 && args[0] == cmdRevParse && args[1] == cmdFlagVerify:
				return "", errGitFailed // BranchExists → false
			}
			return "", nil
		},
		runInDir: noopRunInDir,
	}
	restore := overrideNewRunner(r)
	defer restore()

	cmd, _ := newTestCmd()
	addBranchFlags(cmd)
	cmd.SetContext(config.WithConfig(context.Background(), cfg))

	err := addCmd.RunE(cmd, []string{"branch:" + branchFeatureX})
	if err == nil {
		t.Fatal("expected error for missing branch")
	}
	if !strings.Contains(err.Error(), "does not exist") {
		t.Errorf("error %q should mention 'does not exist'", err)
	}
}

// nonHeadBranchRunInDirFn is the RunInDir closure for TestAddBranchRejectsNonHead.
// HEAD is main, not the feature branch being promoted.
func nonHeadBranchRunInDirFn() func(dir string, args ...string) (string, error) {
	return func(_ string, args ...string) (string, error) {
		if len(args) >= 2 && args[0] == cmdSymbolicRef && args[1] == cmdFlagShort {
			return branchMain, nil
		}
		return "", nil
	}
}

func TestAddBranchRejectsNonHead(t *testing.T) {
	repoDir := t.TempDir()
	cfg := &config.Config{WorktreeDir: "worktrees"}

	r := &mockRunner{
		run:      branchPromoteRunFn(repoDir),
		runInDir: nonHeadBranchRunInDirFn(),
	}
	restore := overrideNewRunner(r)
	defer restore()

	cmd, _ := newTestCmd()
	addBranchFlags(cmd)
	cmd.SetContext(config.WithConfig(context.Background(), cfg))

	err := addCmd.RunE(cmd, []string{"branch:" + branchFeatureX})
	if err == nil {
		t.Fatal("expected error for non-HEAD branch")
	}
	if !strings.Contains(err.Error(), "is not the current branch") {
		t.Errorf("error %q should mention 'is not the current branch'", err)
	}
}

func TestAddBranchRejectsAlreadyInWorktree(t *testing.T) {
	repoDir := t.TempDir()
	cfg := &config.Config{WorktreeDir: "worktrees"}
	otherWtPath := "/some/other/worktree"
	porcelain := wtPrefix + repoDir + headMainBlock + "\nworktree " + otherWtPath + "\nHEAD def\nbranch refs/heads/" + branchFeatureX + "\n"

	r := &mockRunner{
		run: func(args ...string) (string, error) {
			switch {
			case len(args) >= 2 && args[1] == cmdGitCommonDir:
				return filepath.Join(repoDir, ".git"), nil
			case len(args) >= 1 && args[0] == cmdSymbolicRef:
				return refsRemotesOriginMain, nil
			case len(args) >= 2 && args[0] == cmdRevParse && args[1] == cmdFlagVerify:
				return "", nil
			case len(args) >= 2 && args[0] == cmdWorktreeTest && args[1] == cmdList:
				return porcelain, nil
			}
			return "", nil
		},
		runInDir: noopRunInDir,
	}
	restore := overrideNewRunner(r)
	defer restore()

	cmd, _ := newTestCmd()
	addBranchFlags(cmd)
	cmd.SetContext(config.WithConfig(context.Background(), cfg))

	err := addCmd.RunE(cmd, []string{"branch:" + branchFeatureX})
	if err == nil {
		t.Fatal("expected error for branch already in worktree")
	}
	if !strings.Contains(err.Error(), "already checked out in worktree") {
		t.Errorf("error %q should mention 'already checked out in worktree'", err)
	}
}

// pathExistsBranchRunInDirFn is the RunInDir closure for TestAddBranchRejectsPathExists.
// HEAD is the feature branch (validation passes up to the path check).
func pathExistsBranchRunInDirFn() func(dir string, args ...string) (string, error) {
	return func(_ string, args ...string) (string, error) {
		if len(args) >= 2 && args[0] == cmdSymbolicRef && args[1] == cmdFlagShort {
			return branchFeatureX, nil
		}
		return "", nil
	}
}

func TestAddBranchRejectsPathExists(t *testing.T) {
	repoDir := t.TempDir()
	wtDir := filepath.Join(repoDir, "worktrees")
	existingPath := filepath.Join(wtDir, "feature-x")
	_ = os.MkdirAll(existingPath, 0o755)
	cfg := &config.Config{WorktreeDir: "worktrees"}

	r := &mockRunner{
		run:      branchPromoteRunFn(repoDir),
		runInDir: pathExistsBranchRunInDirFn(),
	}
	restore := overrideNewRunner(r)
	defer restore()

	cmd, _ := newTestCmd()
	addBranchFlags(cmd)
	cmd.SetContext(config.WithConfig(context.Background(), cfg))

	err := addCmd.RunE(cmd, []string{"branch:" + branchFeatureX})
	if err == nil {
		t.Fatal("expected error for existing path")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Errorf("error %q should mention 'already exists'", err)
	}
}

// conflictRunInDirStash handles stash ops with apply-conflict simulation; returns (out, err, matched).
func conflictRunInDirStash(stashDropped *bool, args []string) (string, error, bool) {
	switch {
	case len(args) >= 3 && args[0] == cmdStash && args[1] == gitSubcmdStashPush:
		return "", nil, true
	case len(args) >= 2 && args[0] == cmdRevParse && args[1] == "stash@{0}":
		return stashSHACmd, nil, true
	case len(args) >= 2 && args[0] == cmdSwitch:
		return "", nil, true
	case len(args) >= 3 && args[0] == cmdStash && args[1] == gitSubcmdStashAppl:
		return "", errors.New("CONFLICT: merge conflict in file.txt"), true
	case len(args) >= 3 && args[0] == cmdStash && args[1] == gitSubcmdStashDrop:
		*stashDropped = true
		return "", nil, true
	}
	return "", nil, false
}

// conflictBranchRunInDirFn is the RunInDir closure for TestAddBranchStashApplyConflict.
func conflictBranchRunInDirFn(stashDropped *bool) func(dir string, args ...string) (string, error) {
	return func(_ string, args ...string) (string, error) {
		if out, ok := branchRunInDirIdentity(true, args); ok {
			return out, nil
		}
		out, err, _ := conflictRunInDirStash(stashDropped, args)
		return out, err
	}
}

func TestAddBranchStashApplyConflict(t *testing.T) {
	repoDir := t.TempDir()
	wtDir := filepath.Join(repoDir, "worktrees")
	_ = os.MkdirAll(wtDir, 0755)
	cfg := &config.Config{WorktreeDir: "worktrees"}
	stashDropped := false

	r := &mockRunner{
		run:      branchPromoteRunFn(repoDir),
		runInDir: conflictBranchRunInDirFn(&stashDropped),
	}
	restore := overrideNewRunner(r)
	defer restore()

	cmd, _ := newTestCmd()
	addBranchFlags(cmd)
	cmd.SetContext(config.WithConfig(context.Background(), cfg))

	err := addCmd.RunE(cmd, []string{"branch:" + branchFeatureX})
	if err == nil {
		t.Fatal("expected error from stash conflict")
	}
	if !strings.Contains(err.Error(), "stash apply had conflicts") {
		t.Errorf("error %q should mention 'stash apply had conflicts'", err)
	}
	if !strings.Contains(err.Error(), stashSHACmd) {
		t.Errorf("error %q should contain the stash SHA", err)
	}
	if stashDropped {
		t.Error("stash should NOT be dropped on conflict")
	}
}

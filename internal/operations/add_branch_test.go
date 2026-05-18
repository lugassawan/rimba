package operations

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const (
	branchFeatureX    = "feature/x"
	stashSHATest      = "abc123def456"
	gitCmdStash       = "stash"
	gitCmdSwitch      = "switch"
	gitCmdSymbolicRef = "symbolic-ref"
	gitSubcmdPush     = "push"
	gitSubcmdApply    = "apply"
	gitSubcmdDrop     = "drop"
	flagVerifyOpt     = "--verify"
	flagShortOpt      = "--short"
	gitSubcmdList     = "list"

	stashListLine = stashSHATest + " stash@{0}"
)

// promoteRunFn returns a Run closure for the happy-path promote mock.
// It returns refsRemotesOriginMain for symbolic-ref, nil for BranchExists,
// the porcelain worktree list, and creates the worktree dir on worktree add.
func promoteRunFn(repoRoot string, makeWtDir bool) func(args ...string) (string, error) {
	porcelain := "worktree " + repoRoot + "\nHEAD abc\nbranch refs/heads/main\n"
	return func(args ...string) (string, error) {
		switch {
		case len(args) >= 1 && args[0] == gitCmdSymbolicRef:
			return refsRemotesOriginMain, nil
		case len(args) >= 2 && args[0] == cmdRevParse && args[1] == flagVerifyOpt:
			return "", nil // BranchExists → true
		case len(args) >= 2 && args[0] == gitCmdWorktree && args[1] == gitSubcmdList:
			return porcelain, nil
		case len(args) >= 2 && args[0] == gitCmdWorktree && args[1] == gitSubcmdAdd:
			if makeWtDir {
				_ = os.MkdirAll(args[2], 0o755)
			}
			return "", nil
		}
		return "", nil
	}
}

// promoteRunInDirIdentity handles symbolic-ref and status matches; reports result and whether matched.
func promoteRunInDirIdentity(dirty bool, args []string) (string, bool) {
	switch {
	case len(args) >= 2 && args[0] == gitCmdSymbolicRef && args[1] == flagShortOpt:
		return branchFeatureX, true
	case len(args) >= 2 && args[0] == gitCmdStatus && args[1] == "--porcelain":
		if dirty {
			return "M file.txt", true
		}
		return "", true
	}
	return "", false
}

// promoteRunInDirStashA handles push / rev-parse / switch operations.
func promoteRunInDirStashA(args []string) (string, bool) {
	switch {
	case len(args) >= 2 && args[0] == gitCmdStash && args[1] == gitSubcmdPush:
		return "", true
	case len(args) >= 2 && args[0] == cmdRevParse && args[1] == "stash@{0}":
		return stashSHATest, true
	case len(args) >= 2 && args[0] == gitCmdSwitch:
		return "", true
	}
	return "", false
}

// promoteRunInDirStashB handles stash list / apply / drop operations.
func promoteRunInDirStashB(args []string) (string, bool) {
	switch {
	case len(args) >= 3 && args[0] == gitCmdStash && args[1] == gitSubcmdList:
		return stashListLine, true
	case len(args) >= 2 && args[0] == gitCmdStash && args[1] == gitSubcmdApply:
		return "", true
	case len(args) >= 2 && args[0] == gitCmdStash && args[1] == gitSubcmdDrop:
		return "", true
	}
	return "", false
}

// promoteRunInDirStash handles happy-path stash operations; reports result and whether matched.
func promoteRunInDirStash(args []string) (string, bool) {
	if out, ok := promoteRunInDirStashA(args); ok {
		return out, ok
	}
	return promoteRunInDirStashB(args)
}

// promoteRunInDirFn returns a RunInDir closure for the happy-path promote mock.
func promoteRunInDirFn(dirty bool) func(dir string, args ...string) (string, error) {
	return func(_ string, args ...string) (string, error) {
		if out, ok := promoteRunInDirIdentity(dirty, args); ok {
			return out, nil
		}
		out, _ := promoteRunInDirStash(args)
		return out, nil
	}
}

// buildPromoteRunner assembles a happy-path mockRunner for PromoteBranch.
func buildPromoteRunner(t *testing.T, repoRoot string, dirty bool, makeWtDir bool) *mockRunner {
	t.Helper()
	return &mockRunner{
		run:      promoteRunFn(repoRoot, makeWtDir),
		runInDir: promoteRunInDirFn(dirty),
	}
}

func TestPromoteBranchDirty(t *testing.T) {
	repoRoot := t.TempDir()
	wtDir := filepath.Join(t.TempDir(), "worktrees")
	wtPath := filepath.Join(wtDir, "feature-x")

	r := buildPromoteRunner(t, repoRoot, true, true)

	got, err := PromoteBranch(context.Background(), wtDir, r, repoRoot, branchFeatureX)
	if err != nil {
		t.Fatalf("PromoteBranch: %v", err)
	}
	if got != wtPath {
		t.Errorf("got path %q, want %q", got, wtPath)
	}
}

func TestPromoteBranchClean(t *testing.T) {
	repoRoot := t.TempDir()
	wtDir := filepath.Join(t.TempDir(), "worktrees")
	wtPath := filepath.Join(wtDir, "feature-x")

	r := buildPromoteRunner(t, repoRoot, false, true)

	got, err := PromoteBranch(context.Background(), wtDir, r, repoRoot, branchFeatureX)
	if err != nil {
		t.Fatalf("PromoteBranch clean: %v", err)
	}
	if got != wtPath {
		t.Errorf("got path %q, want %q", got, wtPath)
	}
}

func TestPromoteBranchRejectsDefaultBranch(t *testing.T) {
	repoRoot := t.TempDir()
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			if len(args) >= 1 && args[0] == gitCmdSymbolicRef {
				return refsRemotesOriginMain, nil
			}
			return "", nil
		},
		runInDir: noopRunInDir,
	}

	_, err := PromoteBranch(context.Background(), t.TempDir(), r, repoRoot, branchMain)
	if err == nil {
		t.Fatal("expected error for default branch")
	}
	if !strings.Contains(err.Error(), "cannot promote default branch") {
		t.Errorf("error %q should mention 'cannot promote default branch'", err)
	}
}

func TestPromoteBranchRejectsBranchMissing(t *testing.T) {
	repoRoot := t.TempDir()
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			if len(args) >= 1 && args[0] == gitCmdSymbolicRef {
				return refsRemotesOriginMain, nil
			}
			return "", errGitFailed // BranchExists → false
		},
		runInDir: noopRunInDir,
	}

	_, err := PromoteBranch(context.Background(), t.TempDir(), r, repoRoot, branchFeatureX)
	if err == nil {
		t.Fatal("expected error for missing branch")
	}
	if !strings.Contains(err.Error(), "does not exist") {
		t.Errorf("error %q should mention 'does not exist'", err)
	}
}

func TestPromoteBranchRejectsAlreadyInWorktree(t *testing.T) {
	repoRoot := t.TempDir()
	otherWtPath := "/some/other/worktree"
	porcelain := "worktree " + repoRoot + "\nHEAD abc\nbranch refs/heads/main\n\nworktree " + otherWtPath + "\nHEAD def\nbranch refs/heads/" + branchFeatureX + "\n"

	r := &mockRunner{
		run: func(args ...string) (string, error) {
			switch {
			case len(args) >= 1 && args[0] == gitCmdSymbolicRef:
				return refsRemotesOriginMain, nil
			case len(args) >= 2 && args[0] == cmdRevParse && args[1] == flagVerifyOpt:
				return "", nil
			case len(args) >= 1 && args[0] == gitCmdWorktree:
				return porcelain, nil
			}
			return "", nil
		},
		runInDir: noopRunInDir,
	}

	_, err := PromoteBranch(context.Background(), t.TempDir(), r, repoRoot, branchFeatureX)
	if err == nil {
		t.Fatal("expected error for branch already in worktree")
	}
	if !strings.Contains(err.Error(), "already checked out in worktree") {
		t.Errorf("error %q should mention 'already checked out in worktree'", err)
	}
}

// nonHeadRunFn is the Run closure for TestPromoteBranchRejectsNonHead.
func nonHeadRunFn(porcelain string) func(args ...string) (string, error) {
	return func(args ...string) (string, error) {
		switch {
		case len(args) >= 1 && args[0] == gitCmdSymbolicRef:
			return refsRemotesOriginMain, nil
		case len(args) >= 2 && args[0] == cmdRevParse && args[1] == flagVerifyOpt:
			return "", nil
		case len(args) >= 1 && args[0] == gitCmdWorktree:
			return porcelain, nil
		}
		return "", nil
	}
}

func TestPromoteBranchRejectsNonHead(t *testing.T) {
	repoRoot := t.TempDir()
	porcelain := "worktree " + repoRoot + "\nHEAD abc\nbranch refs/heads/main\n"

	r := &mockRunner{
		run: nonHeadRunFn(porcelain),
		runInDir: func(_ string, args ...string) (string, error) {
			if len(args) >= 2 && args[0] == gitCmdSymbolicRef && args[1] == flagShortOpt {
				return branchMain, nil // HEAD is main, not feature/x
			}
			return "", nil
		},
	}

	_, err := PromoteBranch(context.Background(), t.TempDir(), r, repoRoot, branchFeatureX)
	if err == nil {
		t.Fatal("expected error for non-head branch")
	}
	if !strings.Contains(err.Error(), "is not the current branch") {
		t.Errorf("error %q should mention 'is not the current branch'", err)
	}
}

// pathExistsRunFn is the Run closure for TestPromoteBranchRejectsPathExists.
func pathExistsRunFn(porcelain string) func(args ...string) (string, error) {
	return func(args ...string) (string, error) {
		switch {
		case len(args) >= 1 && args[0] == gitCmdSymbolicRef:
			return refsRemotesOriginMain, nil
		case len(args) >= 2 && args[0] == cmdRevParse && args[1] == flagVerifyOpt:
			return "", nil
		case len(args) >= 1 && args[0] == gitCmdWorktree:
			return porcelain, nil
		}
		return "", nil
	}
}

func TestPromoteBranchRejectsPathExists(t *testing.T) {
	repoRoot := t.TempDir()
	wtDir := t.TempDir()
	existingPath := filepath.Join(wtDir, "feature-x")
	if err := os.MkdirAll(existingPath, 0o755); err != nil {
		t.Fatal(err)
	}
	porcelain := "worktree " + repoRoot + "\nHEAD abc\nbranch refs/heads/main\n"

	r := &mockRunner{
		run: pathExistsRunFn(porcelain),
		runInDir: func(_ string, args ...string) (string, error) {
			if len(args) >= 2 && args[0] == gitCmdSymbolicRef && args[1] == flagShortOpt {
				return branchFeatureX, nil
			}
			return "", nil
		},
	}

	_, err := PromoteBranch(context.Background(), wtDir, r, repoRoot, branchFeatureX)
	if err == nil {
		t.Fatal("expected error for existing path")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Errorf("error %q should mention 'already exists'", err)
	}
}

// conflictRunFn is the Run closure for TestPromoteBranchStashApplyConflict.
func conflictRunFn(porcelain, wtDir string) func(args ...string) (string, error) {
	return func(args ...string) (string, error) {
		switch {
		case len(args) >= 1 && args[0] == gitCmdSymbolicRef:
			return refsRemotesOriginMain, nil
		case len(args) >= 2 && args[0] == cmdRevParse && args[1] == flagVerifyOpt:
			return "", nil
		case len(args) >= 2 && args[0] == gitCmdWorktree && args[1] == gitSubcmdList:
			return porcelain, nil
		case len(args) >= 2 && args[0] == gitCmdWorktree && args[1] == gitSubcmdAdd:
			wtPath := filepath.Join(wtDir, "feature-x")
			_ = os.MkdirAll(wtPath, 0o755)
			return "", nil
		}
		return "", nil
	}
}

// conflictStash handles stash ops with apply-conflict simulation; returns (out, err, matched).
func conflictStash(stashDropped *bool, args []string) (string, error, bool) {
	switch {
	case len(args) >= 2 && args[0] == gitCmdStash && args[1] == gitSubcmdPush:
		return "", nil, true
	case len(args) >= 2 && args[0] == cmdRevParse && args[1] == "stash@{0}":
		return stashSHATest, nil, true
	case len(args) >= 2 && args[0] == gitCmdSwitch:
		return "", nil, true
	case len(args) >= 2 && args[0] == gitCmdStash && args[1] == gitSubcmdApply:
		return "", errors.New("CONFLICT: merge conflict"), true
	case len(args) >= 2 && args[0] == gitCmdStash && args[1] == gitSubcmdDrop:
		*stashDropped = true
		return "", nil, true
	}
	return "", nil, false
}

// conflictRunInDirFn is the RunInDir closure for TestPromoteBranchStashApplyConflict.
func conflictRunInDirFn(stashDropped *bool) func(dir string, args ...string) (string, error) {
	return func(_ string, args ...string) (string, error) {
		if out, ok := promoteRunInDirIdentity(true, args); ok {
			return out, nil
		}
		out, err, _ := conflictStash(stashDropped, args)
		return out, err
	}
}

func TestPromoteBranchStashApplyConflict(t *testing.T) {
	repoRoot := t.TempDir()
	wtDir := t.TempDir()
	porcelain := "worktree " + repoRoot + "\nHEAD abc\nbranch refs/heads/main\n"
	stashDropped := false

	r := &mockRunner{
		run:      conflictRunFn(porcelain, wtDir),
		runInDir: conflictRunInDirFn(&stashDropped),
	}

	_, err := PromoteBranch(context.Background(), wtDir, r, repoRoot, branchFeatureX)
	if err == nil {
		t.Fatal("expected error from stash apply conflict")
	}
	if !strings.Contains(err.Error(), "stash apply had conflicts") {
		t.Errorf("error %q should mention 'stash apply had conflicts'", err)
	}
	if !strings.Contains(err.Error(), stashSHATest) {
		t.Errorf("error %q should contain the stash SHA for recovery", err)
	}
	if stashDropped {
		t.Error("stash should NOT be dropped on conflict")
	}
}

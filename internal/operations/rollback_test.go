package operations

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lugassawan/rimba/internal/errhint"
)

const rollbackBranch = "feature/login-copy"

func TestRollbackFailedCreateCleanupSucceeds(t *testing.T) {
	setupErr := errors.New("setup failed")
	cleanupContextLive := false
	r := &ctxAwareMockRunner{
		run: func(ctx context.Context, args ...string) (string, error) {
			if len(args) >= 2 && args[0] == "worktree" && args[1] == "remove" {
				cleanupContextLive = ctx.Err() == nil
			}
			return "", nil
		},
	}

	err := RollbackFailedCreate(r, RollbackParams{
		WtPath:     filepath.Join(t.TempDir(), "worktree"),
		Branch:     rollbackBranch,
		Task:       "login-copy",
		OnProgress: nil,
	}, setupErr)
	assertRollbackErrorsAre(t, err, setupErr)
	if !cleanupContextLive {
		t.Error("cleanup context was cancelled, want live context")
	}
	assertRollbackContains(t, err, "Rollback completed", rollbackBranch)
}

func TestRollbackFailedCreateWorktreeRemovalFails(t *testing.T) {
	setupErr := errors.New("setup failed")
	removeErr := errors.New("worktree removal failed")
	wtPath := filepath.Join(t.TempDir(), "worktree")
	if err := os.MkdirAll(wtPath, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(wtPath, ".git"), []byte("gitdir: /tmp/worktrees/login-copy"), 0o644); err != nil {
		t.Fatal(err)
	}
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			if len(args) >= 2 && args[0] == "worktree" && args[1] == "remove" {
				return "", removeErr
			}
			if len(args) >= 1 && args[0] == "branch" {
				t.Fatal("branch deletion must not run after worktree removal fails")
			}
			return "", nil
		},
		runInDir: noopRunInDir,
	}

	err := RollbackFailedCreate(r, RollbackParams{WtPath: wtPath, Branch: rollbackBranch, Task: "login-copy"}, setupErr)
	assertRollbackErrorsAre(t, err, setupErr, removeErr)
	assertRollbackContains(t, err, "Rollback failed", wtPath, "branch preserved", "Directory remains")
}

func TestRollbackFailedCreateBranchDeletionFails(t *testing.T) {
	setupErr := errors.New("setup failed")
	branchErr := errors.New("branch delete failed")
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			if len(args) >= 1 && args[0] == "branch" {
				return "", branchErr
			}
			if len(args) >= 1 && args[0] == "rev-parse" {
				return "", nil // BranchExists reports true after the failed delete
			}
			return "", nil
		},
		runInDir: noopRunInDir,
	}

	err := RollbackFailedCreate(r, RollbackParams{
		WtPath: filepath.Join(t.TempDir(), "worktree"),
		Branch: rollbackBranch,
		Task:   "login-copy",
	}, setupErr)
	assertRollbackErrorsAre(t, err, setupErr, branchErr)
	assertRollbackContains(t, err, "worktree removed", rollbackBranch, "failed to delete branch")
}

func TestRollbackFailedCreateLeavesDirectoryAndBranch(t *testing.T) {
	setupErr := errors.New("setup failed")
	branchErr := errors.New("branch delete failed")
	wtPath := filepath.Join(t.TempDir(), "$(unexpected)")
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			if len(args) >= 2 && args[0] == "worktree" && args[1] == "remove" {
				return "", errors.New("remove failed")
			}
			if len(args) >= 1 && args[0] == "branch" {
				return "", branchErr
			}
			if len(args) >= 1 && args[0] == "rev-parse" {
				return "", nil
			}
			return "", nil
		},
		runInDir: noopRunInDir,
	}

	err := RollbackFailedCreate(r, RollbackParams{WtPath: wtPath, Branch: rollbackBranch, Task: "login-copy"}, setupErr)
	assertRollbackErrorsAre(t, err, setupErr, branchErr)
	assertRollbackContains(t, err, "directory remains", wtPath, rollbackBranch)
	if strings.Contains(err.Error(), "rm -rf") {
		t.Errorf("error = %q, must not render a shell command", err)
	}
}

func TestRollbackFailedCreatePruneFallbackOnly(t *testing.T) {
	setupErr := errors.New("setup failed")
	wtPath := filepath.Join(t.TempDir(), "worktree")
	branchDeleted := false
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			if len(args) >= 2 && args[0] == "worktree" && args[1] == "remove" {
				return "", errors.New("remove failed")
			}
			if len(args) >= 2 && args[0] == "worktree" && args[1] == "prune" {
				return "", nil // prune fallback succeeds → LeftOnDisk, no error
			}
			if len(args) >= 1 && args[0] == "branch" {
				branchDeleted = true
				return "", nil
			}
			if len(args) >= 1 && args[0] == "rev-parse" {
				return "", errGitFailed // BranchExists false → delete counts as done
			}
			return "", nil
		},
		runInDir: noopRunInDir,
	}

	err := RollbackFailedCreate(r, RollbackParams{WtPath: wtPath, Branch: rollbackBranch, Task: "login-copy"}, setupErr)
	if !branchDeleted {
		t.Error("branch deletion must run after prune fallback")
	}
	assertRollbackErrorsAre(t, err, setupErr)
	assertRollbackContains(t, err, rollbackBranch, "was removed", "directory remains", wtPath)
}

func TestRollbackFailedCreateFailedPruneLeavesDirectory(t *testing.T) {
	setupErr := errors.New("setup failed")
	pruneErr := errors.New("prune failed")
	wtPath := filepath.Join(t.TempDir(), "worktree")
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			if len(args) >= 2 && args[0] == "worktree" && args[1] == "remove" {
				return "", errors.New("remove failed")
			}
			if len(args) >= 2 && args[0] == "worktree" && args[1] == "prune" {
				return "", pruneErr
			}
			if len(args) >= 1 && args[0] == "branch" {
				t.Fatal("branch deletion must not run after failed prune")
			}
			return "", nil
		},
		runInDir: noopRunInDir,
	}

	err := RollbackFailedCreate(r, RollbackParams{WtPath: wtPath, Branch: rollbackBranch, Task: "login-copy"}, setupErr)
	assertRollbackErrorsAre(t, err, setupErr, pruneErr)
	assertRollbackContains(t, err, "Rollback failed", "Directory remains", wtPath, "branch preserved")
}

func TestRollbackFailedCreateStripsStaleHints(t *testing.T) {
	inner := errors.New("permission denied")
	setupErr := errhint.WithFix(
		fmt.Errorf("failed to copy files: %w\nTo retry, manually copy files to: /gone/worktree", inner),
		"rimba remove login-copy",
	)
	r := &mockRunner{
		run:      func(args ...string) (string, error) { return "", nil },
		runInDir: noopRunInDir,
	}

	err := RollbackFailedCreate(r, RollbackParams{
		WtPath: filepath.Join(t.TempDir(), "worktree"),
		Branch: rollbackBranch,
		Task:   "login-copy",
	}, setupErr)
	assertRollbackErrorsAre(t, err, setupErr, inner)
	assertRollbackContains(t, err, "Rollback completed")
	for _, stale := range []string{"To fix:", "To retry,", "rimba remove"} {
		if strings.Contains(err.Error(), stale) {
			t.Errorf("error = %q, stale hint %q must be stripped", err, stale)
		}
	}
}

func TestRollbackFailedCreateKeepBranchPreservesBranch(t *testing.T) {
	setupErr := errors.New("setup failed")
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			if len(args) >= 1 && args[0] == "branch" {
				t.Fatal("branch deletion must not run when KeepBranch is set")
			}
			return "", nil
		},
		runInDir: noopRunInDir,
	}

	err := RollbackFailedCreate(r, RollbackParams{
		WtPath:     filepath.Join(t.TempDir(), "worktree"),
		Branch:     rollbackBranch,
		Task:       "login-copy",
		KeepBranch: true,
	}, setupErr)
	assertRollbackErrorsAre(t, err, setupErr)
	assertRollbackContains(t, err, "Rollback completed", "branch", rollbackBranch, "preserved")
}

func TestAddWorktreeRollsBackOnSetupFailure(t *testing.T) {
	listErr := errors.New("worktree list failed")
	wtDir := filepath.Join(t.TempDir(), "worktrees")
	worktreeRemoved, branchDeleted := false, false

	_, err := AddWorktree(context.Background(), newAddRollbackRunner(listErr, &worktreeRemoved, &branchDeleted), AddParams{
		Task:        "login-copy",
		Prefix:      "feature/",
		Source:      branchMain,
		RepoRoot:    t.TempDir(),
		WorktreeDir: wtDir,
		SkipHooks:   true,
	}, nil)
	if err == nil {
		t.Fatal("expected setup failure error, got nil")
	}
	if !worktreeRemoved {
		t.Error("rollback must remove the created worktree")
	}
	if !branchDeleted {
		t.Error("rollback must delete the created branch")
	}
	assertRollbackErrorsAre(t, err, listErr)
	assertRollbackContains(t, err, "Rollback completed")
}

// newAddRollbackRunner mocks the AddWorktree failure path: creation succeeds,
// the PostCreateSetup deps stage fails with listErr, and the rollback removal
// and branch deletion succeed while recording both in the out-flags.
func newAddRollbackRunner(listErr error, worktreeRemoved, branchDeleted *bool) *mockRunner {
	return &mockRunner{
		run: func(args ...string) (string, error) {
			switch {
			case len(args) >= 2 && args[0] == "worktree" && args[1] == "add":
				return "", nil
			case len(args) >= 2 && args[0] == "worktree" && args[1] == "list":
				return "", listErr
			case len(args) >= 2 && args[0] == "worktree" && args[1] == "remove":
				*worktreeRemoved = true
				return "", nil
			case len(args) >= 2 && args[0] == "branch" && args[1] == "-D":
				*branchDeleted = true
				return "", nil
			case len(args) >= 1 && args[0] == "rev-parse":
				return "", errGitFailed // BranchExists false
			}
			return "", nil
		},
		runInDir: noopRunInDir,
	}
}

func assertRollbackErrorsAre(t *testing.T, err error, wants ...error) {
	t.Helper()
	for _, want := range wants {
		if !errors.Is(err, want) {
			t.Errorf("errors.Is(err, %v) = false, err = %v", want, err)
		}
	}
}

func assertRollbackContains(t *testing.T, err error, wants ...string) {
	t.Helper()
	for _, want := range wants {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error = %q, want substring %q", err, want)
		}
	}
}

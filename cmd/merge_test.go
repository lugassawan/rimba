package cmd

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/lugassawan/rimba/internal/config"
	"github.com/lugassawan/rimba/internal/operations"
	"github.com/lugassawan/rimba/testutil"
)

const (
	mergeWorktreeOut = "worktree /repo\nHEAD abc123\nbranch refs/heads/main\n\nworktree /wt/feature-login\nHEAD def456\nbranch refs/heads/feature/login\n\nworktree /wt/feature-dashboard\nHEAD ghi789\nbranch refs/heads/feature/dashboard\n"
	gitCmdPush       = "push" // first arg of "git push <remote> ..." — distinct from gitSubcmdStashPush which is args[1] of "git stash push"
)

func mergeTestRunner(mergeErr error) *mockRunner {
	return &mockRunner{
		run: func(args ...string) (string, error) {
			if len(args) >= 2 && args[1] == cmdShowToplevel {
				return repoPath, nil
			}
			return mergeWorktreeOut, nil
		},
		runInDir: func(_ string, args ...string) (string, error) {
			if len(args) >= 1 && args[0] == cmdStatus {
				return "", nil
			}
			if len(args) >= 1 && args[0] == flagSyncMerge {
				return "", mergeErr
			}
			return "", nil
		},
	}
}

func TestMergeIntoMainSuccess(t *testing.T) {
	cfg := &config.Config{DefaultSource: branchMain, WorktreeDir: defaultRelativeWtDir}
	r := mergeTestRunner(nil)
	restore := overrideNewRunner(r)
	defer restore()

	cmd, buf := newTestCmd()
	cmd.Flags().String(flagInto, "", "")
	cmd.Flags().Bool(flagNoFF, false, "")
	cmd.Flags().Bool(flagKeep, false, "")
	cmd.Flags().Bool(flagDelete, false, "")
	cmd.SetContext(config.WithConfig(context.Background(), cfg))

	err := mergeCmd.RunE(cmd, []string{"login"})
	if err != nil {
		t.Fatalf(fatalMergeRunE, err)
	}
	out := buf.String()
	if !strings.Contains(out, "Merged feature/login into main") {
		t.Errorf("output = %q, want merge message", out)
	}
}

func TestMergeOrphanedSourceHardErrors(t *testing.T) {
	// Only "TASK-" is configured, so the "PROJ-*" source branch is orphaned;
	// merge has no --force flag, so this guard can never be bypassed here.
	cfg := &config.Config{
		DefaultSource: branchMain,
		WorktreeDir:   defaultRelativeWtDir,
		Resolver: &config.ResolverConfig{
			Prefix: []config.PrefixEntry{{Prefix: "TASK-"}},
		},
	}
	worktreeOut := orphanedProjWorktreeOut
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			if len(args) >= 2 && args[1] == cmdShowToplevel {
				return repoPath, nil
			}
			return worktreeOut, nil
		},
		runInDir: noopRunInDir,
	}
	restore := overrideNewRunner(r)
	defer restore()

	cmd, _ := newTestCmd()
	cmd.Flags().String(flagInto, "", "")
	cmd.Flags().Bool(flagNoFF, false, "")
	cmd.Flags().Bool(flagKeep, false, "")
	cmd.Flags().Bool(flagDelete, false, "")
	cmd.SetContext(config.WithConfig(context.Background(), cfg))

	err := mergeCmd.RunE(cmd, []string{"PROJ-123"})
	if err == nil {
		t.Fatal("expected orphan-guard error, got nil")
	}
	if !strings.Contains(err.Error(), "re-add the prefix") {
		t.Errorf("error = %q, want it to mention re-adding the prefix", err.Error())
	}
}

func TestMergeKeep(t *testing.T) {
	cfg := &config.Config{DefaultSource: branchMain, WorktreeDir: defaultRelativeWtDir}
	r := mergeTestRunner(nil)
	restore := overrideNewRunner(r)
	defer restore()

	cmd, buf := newTestCmd()
	cmd.Flags().String(flagInto, "", "")
	cmd.Flags().Bool(flagNoFF, false, "")
	cmd.Flags().Bool(flagKeep, false, "")
	cmd.Flags().Bool(flagDelete, false, "")
	_ = cmd.Flags().Set(flagKeep, "true")
	cmd.SetContext(config.WithConfig(context.Background(), cfg))

	err := mergeCmd.RunE(cmd, []string{"login"})
	if err != nil {
		t.Fatalf(fatalMergeRunE, err)
	}
	out := buf.String()
	if strings.Contains(out, msgRemovedWorktree) {
		t.Errorf("output = %q, should not remove with --keep", out)
	}
}

func TestMergeIntoWorktree(t *testing.T) {
	cfg := &config.Config{DefaultSource: branchMain, WorktreeDir: defaultRelativeWtDir}
	r := mergeTestRunner(nil)
	restore := overrideNewRunner(r)
	defer restore()

	cmd, buf := newTestCmd()
	cmd.Flags().String(flagInto, "", "")
	cmd.Flags().Bool(flagNoFF, false, "")
	cmd.Flags().Bool(flagKeep, false, "")
	cmd.Flags().Bool(flagDelete, false, "")
	_ = cmd.Flags().Set(flagInto, "dashboard")
	cmd.SetContext(config.WithConfig(context.Background(), cfg))

	err := mergeCmd.RunE(cmd, []string{"login"})
	if err != nil {
		t.Fatalf(fatalMergeRunE, err)
	}
	out := buf.String()
	if !strings.Contains(out, "Merged feature/login into feature/dashboard") {
		t.Errorf("output = %q, want merge into worktree message", out)
	}
	// By default, source is NOT deleted when merging into another worktree
	if strings.Contains(out, msgRemovedWorktree) {
		t.Errorf("output = %q, should not remove source when merging into worktree without --delete", out)
	}
}

func TestMergeSourceDirty(t *testing.T) {
	cfg := &config.Config{DefaultSource: branchMain, WorktreeDir: defaultRelativeWtDir}
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			if len(args) >= 2 && args[1] == cmdShowToplevel {
				return repoPath, nil
			}
			return mergeWorktreeOut, nil
		},
		runInDir: func(dir string, args ...string) (string, error) {
			if len(args) >= 1 && args[0] == cmdStatus {
				if strings.Contains(dir, "login") {
					return dirtyOutput, nil
				}
				return "", nil
			}
			return "", nil
		},
	}
	restore := overrideNewRunner(r)
	defer restore()

	cmd, _ := newTestCmd()
	cmd.Flags().String(flagInto, "", "")
	cmd.Flags().Bool(flagNoFF, false, "")
	cmd.Flags().Bool(flagKeep, false, "")
	cmd.Flags().Bool(flagDelete, false, "")
	cmd.SetContext(config.WithConfig(context.Background(), cfg))

	err := mergeCmd.RunE(cmd, []string{"login"})
	if err == nil {
		t.Fatal("expected error for dirty source")
	}
	if !strings.Contains(err.Error(), "uncommitted changes") {
		t.Errorf("error = %q, want 'uncommitted changes'", err.Error())
	}
}

func TestMergeTargetDirty(t *testing.T) {
	cfg := &config.Config{DefaultSource: branchMain, WorktreeDir: defaultRelativeWtDir}
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			if len(args) >= 2 && args[1] == cmdGitCommonDir {
				return repoPath + "/.git", nil
			}
			if len(args) >= 2 && args[1] == cmdShowToplevel {
				return repoPath, nil
			}
			return mergeWorktreeOut, nil
		},
		runInDir: func(dir string, args ...string) (string, error) {
			if len(args) >= 1 && args[0] == cmdStatus {
				if dir == repoPath {
					return dirtyOutput, nil
				}
				return "", nil
			}
			return "", nil
		},
	}
	restore := overrideNewRunner(r)
	defer restore()

	cmd, _ := newTestCmd()
	cmd.Flags().String(flagInto, "", "")
	cmd.Flags().Bool(flagNoFF, false, "")
	cmd.Flags().Bool(flagKeep, false, "")
	cmd.Flags().Bool(flagDelete, false, "")
	cmd.SetContext(config.WithConfig(context.Background(), cfg))

	err := mergeCmd.RunE(cmd, []string{"login"})
	if err == nil {
		t.Fatal("expected error for dirty target")
	}
}

func TestMergeFails(t *testing.T) {
	cfg := &config.Config{DefaultSource: branchMain, WorktreeDir: defaultRelativeWtDir}
	r := mergeTestRunner(errors.New("conflict"))
	restore := overrideNewRunner(r)
	defer restore()

	cmd, _ := newTestCmd()
	cmd.Flags().String(flagInto, "", "")
	cmd.Flags().Bool(flagNoFF, false, "")
	cmd.Flags().Bool(flagKeep, false, "")
	cmd.Flags().Bool(flagDelete, false, "")
	cmd.SetContext(config.WithConfig(context.Background(), cfg))

	err := mergeCmd.RunE(cmd, []string{"login"})
	if err == nil {
		t.Fatal("expected error from merge failure")
	}
	if !strings.Contains(err.Error(), "merge failed") {
		t.Errorf("error = %q, want 'merge failed'", err.Error())
	}
}

func TestMergeSourceNotFound(t *testing.T) {
	cfg := &config.Config{DefaultSource: branchMain, WorktreeDir: defaultRelativeWtDir}
	r := mergeTestRunner(nil)
	restore := overrideNewRunner(r)
	defer restore()

	cmd, _ := newTestCmd()
	cmd.Flags().String(flagInto, "", "")
	cmd.Flags().Bool(flagNoFF, false, "")
	cmd.Flags().Bool(flagKeep, false, "")
	cmd.Flags().Bool(flagDelete, false, "")
	cmd.SetContext(config.WithConfig(context.Background(), cfg))

	err := mergeCmd.RunE(cmd, []string{"nonexistent"})
	if err == nil {
		t.Fatal("expected error for missing source")
	}
}

func TestMergeRemoveWorktreeFails(t *testing.T) {
	cfg := &config.Config{DefaultSource: branchMain, WorktreeDir: defaultRelativeWtDir}
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			if len(args) >= 2 && args[1] == cmdShowToplevel {
				return repoPath, nil
			}
			if len(args) >= 2 && args[0] == cmdWorktreeTest && args[1] == cmdRemove {
				return "", errors.New("locked")
			}
			return mergeWorktreeOut, nil
		},
		runInDir: noopRunInDir,
	}
	restore := overrideNewRunner(r)
	defer restore()

	cmd, buf := newTestCmd()
	cmd.Flags().String(flagInto, "", "")
	cmd.Flags().Bool(flagNoFF, false, "")
	cmd.Flags().Bool(flagKeep, false, "")
	cmd.Flags().Bool(flagDelete, false, "")
	cmd.SetContext(config.WithConfig(context.Background(), cfg))

	err := mergeCmd.RunE(cmd, []string{"login"})
	if err != nil {
		t.Fatalf("expected no error (remove failure is non-fatal), got: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "failed to remove worktree") {
		t.Errorf("output = %q, want 'failed to remove worktree'", out)
	}
	if !strings.Contains(out, "To remove manually: git worktree remove --force -- /wt/feature-login") {
		t.Errorf("output = %q, want a path-based 'git worktree remove --force --' hint", out)
	}
}

// TestMergeRemoveWorktreePrunablePruneFails guards #374's merge-side hint:
// when the source is prunable and its recovery prune itself fails, the
// printed hint must be `git worktree prune`, not the doomed
// `git worktree remove --force` that caused the original bug.
func TestMergeRemoveWorktreePrunablePruneFails(t *testing.T) {
	prunableWorktreeOut := "worktree /repo\nHEAD abc123\nbranch refs/heads/main\n\n" +
		"worktree /wt/feature-login\nHEAD def456\nbranch refs/heads/feature/login\nprunable gitdir file points to non-existent location\n"

	cfg := &config.Config{DefaultSource: branchMain, WorktreeDir: defaultRelativeWtDir}
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			if len(args) >= 2 && args[1] == cmdShowToplevel {
				return repoPath, nil
			}
			if len(args) >= 2 && args[0] == cmdWorktreeTest && args[1] == cmdPrune {
				return "", errors.New("prune failed")
			}
			return prunableWorktreeOut, nil
		},
		runInDir: noopRunInDir,
	}
	restore := overrideNewRunner(r)
	defer restore()

	cmd, buf := newTestCmd()
	cmd.Flags().String(flagInto, "", "")
	cmd.Flags().Bool(flagNoFF, false, "")
	cmd.Flags().Bool(flagKeep, false, "")
	cmd.Flags().Bool(flagDelete, false, "")
	cmd.SetContext(config.WithConfig(context.Background(), cfg))

	err := mergeCmd.RunE(cmd, []string{"login"})
	if err != nil {
		t.Fatalf("expected no error (remove failure is non-fatal), got: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "failed to remove worktree") {
		t.Errorf("output = %q, want 'failed to remove worktree'", out)
	}
	if !strings.Contains(out, "To remove manually: git worktree prune") {
		t.Errorf("output = %q, want a 'git worktree prune' hint", out)
	}
}

// TestMergePrunableSourceSuccessMessage guards a follow-up to #374: once a
// prunable source's dirty check is skipped, a merge with auto-cleanup can
// actually succeed and reach the success-path print — which must use the
// same "Cleared stale worktree registration" wording as clean/remove, not
// the misleading "Removed worktree" (git worktree prune leaves the
// directory on disk).
func TestMergePrunableSourceSuccessMessage(t *testing.T) {
	prunableWorktreeOut := "worktree /repo\nHEAD abc123\nbranch refs/heads/main\n\n" +
		"worktree /wt/feature-login\nHEAD def456\nbranch refs/heads/feature/login\nprunable gitdir file points to non-existent location\n"

	cfg := &config.Config{DefaultSource: branchMain, WorktreeDir: defaultRelativeWtDir}
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			if len(args) >= 2 && args[1] == cmdShowToplevel {
				return repoPath, nil
			}
			return prunableWorktreeOut, nil
		},
		runInDir: func(_ string, args ...string) (string, error) {
			if len(args) >= 1 && (args[0] == cmdStatus || args[0] == flagSyncMerge) {
				return "", nil
			}
			return "", nil
		},
	}
	restore := overrideNewRunner(r)
	defer restore()

	cmd, buf := newTestCmd()
	cmd.Flags().String(flagInto, "", "")
	cmd.Flags().Bool(flagNoFF, false, "")
	cmd.Flags().Bool(flagKeep, false, "")
	cmd.Flags().Bool(flagDelete, false, "")
	cmd.SetContext(config.WithConfig(context.Background(), cfg))

	err := mergeCmd.RunE(cmd, []string{"login"})
	if err != nil {
		t.Fatalf(fatalMergeRunE, err)
	}
	out := buf.String()
	if !strings.Contains(out, "Cleared stale worktree registration: /wt/feature-login") {
		t.Errorf("output = %q, want a distinct prunable-recovery message, not 'Removed worktree'", out)
	}
	if strings.Contains(out, "Removed worktree") {
		t.Errorf("output = %q, want NOT 'Removed worktree' for the prunable-recovery path", out)
	}
}

func TestMergeIntoWorktreeWithDelete(t *testing.T) {
	cfg := &config.Config{DefaultSource: branchMain, WorktreeDir: defaultRelativeWtDir}
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			if len(args) >= 2 && args[1] == cmdShowToplevel {
				return repoPath, nil
			}
			// worktree remove succeeds
			if len(args) >= 2 && args[0] == cmdWorktreeTest && args[1] == cmdRemove {
				return "", nil
			}
			// branch -D succeeds
			if len(args) >= 1 && args[0] == cmdBranch {
				return "", nil
			}
			return mergeWorktreeOut, nil
		},
		runInDir: noopRunInDir,
	}
	restore := overrideNewRunner(r)
	defer restore()

	cmd, buf := newTestCmd()
	cmd.Flags().String(flagInto, "", "")
	cmd.Flags().Bool(flagNoFF, false, "")
	cmd.Flags().Bool(flagKeep, false, "")
	cmd.Flags().Bool(flagDelete, false, "")
	_ = cmd.Flags().Set(flagInto, "dashboard")
	_ = cmd.Flags().Set(flagDelete, "true")
	cmd.SetContext(config.WithConfig(context.Background(), cfg))

	err := mergeCmd.RunE(cmd, []string{"login"})
	if err != nil {
		t.Fatalf(fatalMergeRunE, err)
	}
	out := buf.String()
	if !strings.Contains(out, msgRemovedWorktree) {
		t.Errorf("output = %q, want 'Removed worktree'", out)
	}
	if !strings.Contains(out, "Deleted branch") {
		t.Errorf("output = %q, want 'Deleted branch'", out)
	}
}

func TestMergeIntoTargetNotFound(t *testing.T) {
	cfg := &config.Config{DefaultSource: branchMain, WorktreeDir: defaultRelativeWtDir}
	r := mergeTestRunner(nil)
	restore := overrideNewRunner(r)
	defer restore()

	cmd, _ := newTestCmd()
	cmd.Flags().String(flagInto, "", "")
	cmd.Flags().Bool(flagNoFF, false, "")
	cmd.Flags().Bool(flagKeep, false, "")
	cmd.Flags().Bool(flagDelete, false, "")
	_ = cmd.Flags().Set(flagInto, "nonexistent")
	cmd.SetContext(config.WithConfig(context.Background(), cfg))

	err := mergeCmd.RunE(cmd, []string{"login"})
	if err == nil {
		t.Fatal("expected error for nonexistent target")
	}
	if !strings.Contains(err.Error(), "worktree not found") {
		t.Errorf("error = %q, want 'worktree not found'", err.Error())
	}
}

func TestMergeDeleteBranchFails(t *testing.T) {
	cfg := &config.Config{DefaultSource: branchMain, WorktreeDir: defaultRelativeWtDir}
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			if len(args) >= 2 && args[1] == cmdShowToplevel {
				return repoPath, nil
			}
			// worktree remove succeeds
			if len(args) >= 2 && args[0] == cmdWorktreeTest && args[1] == cmdRemove {
				return "", nil
			}
			// branch delete fails
			if len(args) >= 1 && args[0] == cmdBranch {
				return "", errors.New("branch delete error")
			}
			return mergeWorktreeOut, nil
		},
		runInDir: noopRunInDir,
	}
	restore := overrideNewRunner(r)
	defer restore()

	cmd, buf := newTestCmd()
	cmd.Flags().String(flagInto, "", "")
	cmd.Flags().Bool(flagNoFF, false, "")
	cmd.Flags().Bool(flagKeep, false, "")
	cmd.Flags().Bool(flagDelete, false, "")
	cmd.SetContext(config.WithConfig(context.Background(), cfg))

	err := mergeCmd.RunE(cmd, []string{"login"})
	if err != nil {
		t.Fatalf("expected no error (branch delete failure is non-fatal), got: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "failed to delete branch") {
		t.Errorf("output = %q, want 'failed to delete branch'", out)
	}
}

func TestMergeTargetDirtyInto(t *testing.T) {
	cfg := &config.Config{DefaultSource: branchMain, WorktreeDir: defaultRelativeWtDir}
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			if len(args) >= 2 && args[1] == cmdShowToplevel {
				return repoPath, nil
			}
			return mergeWorktreeOut, nil
		},
		runInDir: func(dir string, args ...string) (string, error) {
			if len(args) >= 1 && args[0] == cmdStatus {
				// dashboard worktree is dirty
				if strings.Contains(dir, "dashboard") {
					return dirtyOutput, nil
				}
				return "", nil
			}
			return "", nil
		},
	}
	restore := overrideNewRunner(r)
	defer restore()

	cmd, _ := newTestCmd()
	cmd.Flags().String(flagInto, "", "")
	cmd.Flags().Bool(flagNoFF, false, "")
	cmd.Flags().Bool(flagKeep, false, "")
	cmd.Flags().Bool(flagDelete, false, "")
	_ = cmd.Flags().Set(flagInto, "dashboard")
	cmd.SetContext(config.WithConfig(context.Background(), cfg))

	err := mergeCmd.RunE(cmd, []string{"login"})
	if err == nil {
		t.Fatal("expected error for dirty target worktree")
	}
	if !strings.Contains(err.Error(), "uncommitted changes") {
		t.Errorf("error = %q, want 'uncommitted changes'", err.Error())
	}
}

func TestMergeDryRun(t *testing.T) {
	cfg := &config.Config{DefaultSource: branchMain, WorktreeDir: defaultRelativeWtDir}
	r := mergeTestRunner(nil)
	restore := overrideNewRunner(r)
	defer restore()

	cmd, buf := newTestCmd()
	cmd.Flags().String(flagInto, "", "")
	cmd.Flags().Bool(flagNoFF, false, "")
	cmd.Flags().Bool(flagKeep, false, "")
	cmd.Flags().Bool(flagDelete, false, "")
	cmd.Flags().Bool(flagDryRun, false, "")
	_ = cmd.Flags().Set(flagDryRun, "true")
	cmd.SetContext(config.WithConfig(context.Background(), cfg))

	if err := mergeCmd.RunE(cmd, []string{"login"}); err != nil {
		t.Fatalf(fatalMergeRunE, err)
	}
	out := buf.String()
	if !strings.Contains(out, "[dry-run]") {
		t.Errorf("output = %q, want '[dry-run]' prefix", out)
	}
	if strings.Contains(out, "Merged feature/login") {
		t.Errorf("output = %q, must not contain 'Merged' in dry-run mode", out)
	}
}

// mergeTestRunnerWithCommonDir is mergeTestRunner plus a real, controlled
// --git-common-dir response, so a planted sweep manifest + lock under
// commonDir is where reapConfidentLocks will actually look.
func mergeTestRunnerWithCommonDir(commonDir string) *mockRunner {
	return &mockRunner{
		run: func(args ...string) (string, error) {
			if len(args) >= 2 && args[1] == cmdGitCommonDir {
				return commonDir, nil
			}
			if len(args) >= 2 && args[1] == cmdShowToplevel {
				return repoPath, nil
			}
			return mergeWorktreeOut, nil
		},
		runInDir: func(_ string, args ...string) (string, error) {
			if len(args) >= 1 && args[0] == cmdStatus {
				return "", nil
			}
			return "", nil
		},
	}
}

// TestMergeDryRunSkipsConfidentReap guards against a confident reap (a real
// os.Remove) running as a side effect of a documented no-op preview: --dry-run
// must not touch a lock even when its sweep manifest proves the owner is dead.
func TestMergeDryRunSkipsConfidentReap(t *testing.T) {
	commonDir := t.TempDir()
	lockPath := writeLockFileWithAge(t, commonDir, operations.MinLockAge+time.Second)
	adminDir := filepath.Dir(lockPath)
	plantSweepManifest(t, commonDir, testutil.DeadPID(t), []string{adminDir})

	cfg := &config.Config{DefaultSource: branchMain, WorktreeDir: defaultRelativeWtDir}
	r := mergeTestRunnerWithCommonDir(commonDir)
	restore := overrideNewRunner(r)
	defer restore()

	cmd, buf := newTestCmd()
	cmd.Flags().String(flagInto, "", "")
	cmd.Flags().Bool(flagNoFF, false, "")
	cmd.Flags().Bool(flagKeep, false, "")
	cmd.Flags().Bool(flagDelete, false, "")
	cmd.Flags().Bool(flagDryRun, false, "")
	_ = cmd.Flags().Set(flagDryRun, "true")
	cmd.SetContext(config.WithConfig(context.Background(), cfg))

	if err := mergeCmd.RunE(cmd, []string{"login"}); err != nil {
		t.Fatalf(fatalMergeRunE, err)
	}
	out := buf.String()
	if strings.Contains(out, "Recovered") {
		t.Errorf("output = %q, want no recovery notice under --dry-run", out)
	}
	if _, err := os.Stat(lockPath); err != nil {
		t.Error("expected lock to remain untouched under --dry-run")
	}
}

// TestMergeNonDryRunPerformsConfidentReap is the counterpart: outside
// --dry-run, the same confidently-dead-owner lock is recovered.
func TestMergeNonDryRunPerformsConfidentReap(t *testing.T) {
	commonDir := t.TempDir()
	lockPath := writeLockFileWithAge(t, commonDir, operations.MinLockAge+time.Second)
	adminDir := filepath.Dir(lockPath)
	plantSweepManifest(t, commonDir, testutil.DeadPID(t), []string{adminDir})

	cfg := &config.Config{DefaultSource: branchMain, WorktreeDir: defaultRelativeWtDir}
	r := mergeTestRunnerWithCommonDir(commonDir)
	restore := overrideNewRunner(r)
	defer restore()

	cmd, buf := newTestCmd()
	cmd.Flags().String(flagInto, "", "")
	cmd.Flags().Bool(flagNoFF, false, "")
	cmd.Flags().Bool(flagKeep, true, "") // keep the source worktree — the merge machinery itself is not under test here
	cmd.Flags().Bool(flagDelete, false, "")
	cmd.Flags().Bool(flagDryRun, false, "")
	cmd.SetContext(config.WithConfig(context.Background(), cfg))

	if err := mergeCmd.RunE(cmd, []string{"login"}); err != nil {
		t.Fatalf(fatalMergeRunE, err)
	}
	out := buf.String()
	if !strings.Contains(out, "Recovered 1 stale index.lock file(s)") {
		t.Errorf("output = %q, want a recovery notice", out)
	}
	if _, err := os.Stat(lockPath); !os.IsNotExist(err) {
		t.Error("expected the dead-owner lock to be removed")
	}
}

func TestMergeWithNoFF(t *testing.T) {
	cfg := &config.Config{DefaultSource: branchMain, WorktreeDir: defaultRelativeWtDir}

	var mergeArgs []string
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			if len(args) >= 2 && args[1] == cmdShowToplevel {
				return repoPath, nil
			}
			return mergeWorktreeOut, nil
		},
		runInDir: func(_ string, args ...string) (string, error) {
			if len(args) >= 1 && args[0] == cmdStatus {
				return "", nil
			}
			if len(args) >= 1 && args[0] == flagSyncMerge {
				mergeArgs = append([]string{}, args...)
				return "", nil
			}
			return "", nil
		},
	}
	restore := overrideNewRunner(r)
	defer restore()

	cmd, buf := newTestCmd()
	cmd.Flags().String(flagInto, "", "")
	cmd.Flags().Bool(flagNoFF, false, "")
	cmd.Flags().Bool(flagKeep, false, "")
	cmd.Flags().Bool(flagDelete, false, "")
	_ = cmd.Flags().Set(flagNoFF, "true")
	_ = cmd.Flags().Set(flagKeep, "true")
	cmd.SetContext(config.WithConfig(context.Background(), cfg))

	err := mergeCmd.RunE(cmd, []string{"login"})
	if err != nil {
		t.Fatalf(fatalMergeRunE, err)
	}
	out := buf.String()
	if !strings.Contains(out, "Merged feature/login into main") {
		t.Errorf("output = %q, want merge message", out)
	}

	// Verify --no-ff was passed to merge
	if !slices.Contains(mergeArgs, "--no-ff") {
		t.Errorf("merge args = %v, want --no-ff to be present", mergeArgs)
	}
}

func TestMergeRemoteDeletedOutput(t *testing.T) {
	cfg := &config.Config{DefaultSource: branchMain, WorktreeDir: defaultRelativeWtDir}
	// mergeTestRunner returns ("", nil) for unmatched run calls → origin present, push succeeds.
	r := mergeTestRunner(nil)
	restore := overrideNewRunner(r)
	defer restore()

	cmd, buf := newTestCmd()
	cmd.Flags().String(flagInto, "", "")
	cmd.Flags().Bool(flagNoFF, false, "")
	cmd.Flags().Bool(flagKeep, false, "")
	cmd.Flags().Bool(flagDelete, false, "")
	cmd.SetContext(config.WithConfig(context.Background(), cfg))

	if err := mergeCmd.RunE(cmd, []string{"login"}); err != nil {
		t.Fatalf(fatalMergeRunE, err)
	}
	out := buf.String()
	if !strings.Contains(out, "Deleted remote branch: origin/feature/login") {
		t.Errorf("output = %q, want 'Deleted remote branch: origin/feature/login'", out)
	}
}

func TestMergeRemoteDeleteFailedOutput(t *testing.T) {
	cfg := &config.Config{DefaultSource: branchMain, WorktreeDir: defaultRelativeWtDir}
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			if len(args) >= 2 && args[1] == cmdShowToplevel {
				return repoPath, nil
			}
			if len(args) >= 1 && args[0] == "remote" {
				return "https://github.com/lugassawan/rimba.git", nil
			}
			if len(args) >= 1 && args[0] == gitCmdPush {
				return "", errors.New("connection refused")
			}
			return mergeWorktreeOut, nil
		},
		runInDir: func(_ string, args ...string) (string, error) {
			if len(args) >= 1 && args[0] == cmdStatus {
				return "", nil
			}
			if len(args) >= 1 && args[0] == flagSyncMerge {
				return "", nil
			}
			return "", nil
		},
	}
	restore := overrideNewRunner(r)
	defer restore()

	cmd, buf := newTestCmd()
	cmd.Flags().String(flagInto, "", "")
	cmd.Flags().Bool(flagNoFF, false, "")
	cmd.Flags().Bool(flagKeep, false, "")
	cmd.Flags().Bool(flagDelete, false, "")
	cmd.SetContext(config.WithConfig(context.Background(), cfg))

	if err := mergeCmd.RunE(cmd, []string{"login"}); err != nil {
		t.Fatalf(fatalMergeRunE, err)
	}
	out := buf.String()
	if !strings.Contains(out, "Failed to delete remote branch") {
		t.Errorf("output = %q, want 'Failed to delete remote branch'", out)
	}
	if !strings.Contains(out, "connection refused") {
		t.Errorf("output = %q, want remote error reason 'connection refused'", out)
	}
	if !strings.Contains(out, "git push origin --delete") {
		t.Errorf("output = %q, want manual hint with 'git push origin --delete'", out)
	}
}

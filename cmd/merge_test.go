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

const wantMergeCommand = "merge"

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
	// A real .git file makes this a genuine (non-orphaned) failure, so it
	// short-circuits instead of routing through the heal-and-retry path.
	sourceDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(sourceDir, ".git"), []byte("gitdir: /somewhere/.git/worktrees/login\n"), 0o644); err != nil {
		t.Fatalf("failed to create .git fixture: %v", err)
	}
	worktreeOut := "worktree " + repoPath + "\nHEAD abc123\nbranch refs/heads/main\n\n" +
		"worktree " + sourceDir + "\nHEAD def456\nbranch refs/heads/feature/login\n"

	cfg := &config.Config{DefaultSource: branchMain, WorktreeDir: defaultRelativeWtDir}
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			if len(args) >= 2 && args[1] == cmdShowToplevel {
				return repoPath, nil
			}
			if len(args) >= 2 && args[0] == cmdWorktreeTest && args[1] == cmdRemove {
				return "", errors.New("locked")
			}
			return worktreeOut, nil
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
	if !strings.Contains(out, "To remove manually: git worktree remove --force -- "+sourceDir) {
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
			if len(args) >= 2 && args[0] == cmdWorktreeTest && (args[1] == cmdRemove || args[1] == cmdPrune) {
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

// TestMergePrunableSourceHealsAndRemoves guards #405: repair+remove now heal
// a prunable source fully, so the message must say "Removed worktree", not the #374-era "Cleared stale worktree registration".
func TestMergePrunableSourceHealsAndRemoves(t *testing.T) {
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
	if !strings.Contains(out, "Removed worktree: /wt/feature-login") {
		t.Errorf("output = %q, want 'Removed worktree' — repair+remove fully cleared the directory", out)
	}
	if strings.Contains(out, "Cleared stale worktree registration") {
		t.Errorf("output = %q, want NOT 'Cleared stale worktree registration' once repair+remove succeed", out)
	}
}

// TestMergePrunableSourceFallbackMessage: when repair+remove both still fail,
// the prune-only fallback leaves the dir on disk, so the #374-era "Cleared stale worktree registration" wording still applies.
func TestMergePrunableSourceFallbackMessage(t *testing.T) {
	prunableWorktreeOut := "worktree /repo\nHEAD abc123\nbranch refs/heads/main\n\n" +
		"worktree /wt/feature-login\nHEAD def456\nbranch refs/heads/feature/login\nprunable gitdir file points to non-existent location\n"

	cfg := &config.Config{DefaultSource: branchMain, WorktreeDir: defaultRelativeWtDir}
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			if len(args) >= 2 && args[1] == cmdShowToplevel {
				return repoPath, nil
			}
			if len(args) >= 2 && args[0] == cmdWorktreeTest && args[1] == cmdRemove {
				return "", errors.New("remove failed")
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
// --git-common-dir response, so a planted manifest is where the reap looks.
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

func TestMergeJSON(t *testing.T) {
	cfg := &config.Config{DefaultSource: branchMain, WorktreeDir: defaultRelativeWtDir}
	r := mergeTestRunner(nil)
	restore := overrideNewRunner(r)
	defer restore()

	cmd, buf := newTestCmd()
	cmd.Flags().String(flagInto, "", "")
	cmd.Flags().Bool(flagNoFF, false, "")
	cmd.Flags().Bool(flagKeep, false, "")
	cmd.Flags().Bool(flagDelete, false, "")
	_ = cmd.Flags().Set(flagJSON, "true")
	cmd.SetContext(config.WithConfig(context.Background(), cfg))

	if err := mergeCmd.RunE(cmd, []string{"login"}); err != nil {
		t.Fatalf(fatalMergeRunE, err)
	}

	env, data := decodeAddEnvelope(t, buf.Bytes())
	if env.Command != wantMergeCommand {
		t.Errorf("command = %q, want %q", env.Command, wantMergeCommand)
	}
	if data["source_branch"] != "feature/login" {
		t.Errorf("source_branch = %v, want %q", data["source_branch"], "feature/login")
	}
	if data["target_label"] != branchMain {
		t.Errorf("target_label = %v, want %q", data["target_label"], branchMain)
	}
	if data["dry_run"] != false {
		t.Errorf("dry_run = %v, want false", data["dry_run"])
	}
	if data["source_removed"] != true {
		t.Errorf("source_removed = %v, want true", data["source_removed"])
	}
	if data["worktree_removed"] != true {
		t.Errorf("worktree_removed = %v, want true", data["worktree_removed"])
	}
}

func TestMergeDryRunJSON(t *testing.T) {
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
	_ = cmd.Flags().Set(flagJSON, "true")
	cmd.SetContext(config.WithConfig(context.Background(), cfg))

	if err := mergeCmd.RunE(cmd, []string{"login"}); err != nil {
		t.Fatalf(fatalMergeRunE, err)
	}

	env, data := decodeAddEnvelope(t, buf.Bytes())
	if env.Command != wantMergeCommand {
		t.Errorf("command = %q, want %q", env.Command, wantMergeCommand)
	}
	if data["dry_run"] != true {
		t.Errorf("dry_run = %v, want true", data["dry_run"])
	}
	steps, ok := data["steps"].([]any)
	if !ok || len(steps) == 0 {
		t.Errorf("steps = %#v, want a non-empty array", data["steps"])
	}
}

func TestMergeRemoveErrorJSON(t *testing.T) {
	// A real .git file makes this a genuine (non-orphaned) failure, so it
	// short-circuits instead of routing through the heal-and-retry path.
	sourceDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(sourceDir, ".git"), []byte("gitdir: /somewhere/.git/worktrees/login\n"), 0o644); err != nil {
		t.Fatalf("failed to create .git fixture: %v", err)
	}
	worktreeOut := "worktree " + repoPath + "\nHEAD abc123\nbranch refs/heads/main\n\n" +
		"worktree " + sourceDir + "\nHEAD def456\nbranch refs/heads/feature/login\n"

	cfg := &config.Config{DefaultSource: branchMain, WorktreeDir: defaultRelativeWtDir}
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			if len(args) >= 2 && args[1] == cmdShowToplevel {
				return repoPath, nil
			}
			if len(args) >= 2 && args[0] == cmdWorktreeTest && args[1] == cmdRemove {
				return "", errors.New("locked")
			}
			return worktreeOut, nil
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
	_ = cmd.Flags().Set(flagJSON, "true")
	cmd.SetContext(config.WithConfig(context.Background(), cfg))

	err := mergeCmd.RunE(cmd, []string{"login"})
	if err != nil {
		t.Fatalf("expected no error (remove failure is non-fatal), got: %v", err)
	}

	env, data := decodeAddEnvelope(t, buf.Bytes())
	if env.Command != wantMergeCommand {
		t.Errorf("command = %q, want %q", env.Command, wantMergeCommand)
	}
	removeErr, ok := data["remove_error"].(string)
	if !ok || removeErr == "" {
		t.Errorf("remove_error = %v, want a non-empty string", data["remove_error"])
	}
}

package cmd

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/lugassawan/rimba/internal/config"
)

const mergeWorktreeOut = "worktree /repo\nHEAD abc123\nbranch refs/heads/main\n\nworktree /wt/feature-login\nHEAD def456\nbranch refs/heads/feature/login\n\nworktree /wt/feature-dashboard\nHEAD ghi789\nbranch refs/heads/feature/dashboard\n"

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

package cmd

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/lugassawan/rimba/internal/config"
	"github.com/lugassawan/rimba/internal/resolver"
)

const duplicateRollbackBranch = "feature/login-copy"

func TestDuplicateDefaultBranchError(t *testing.T) {
	repoDir := t.TempDir()
	cfg := &config.Config{DefaultSource: branchMain, WorktreeDir: defaultRelativeWtDir}

	worktreeOut := wtPrefix + repoDir + headMainBlock

	r := &mockRunner{
		run: func(args ...string) (string, error) {
			if len(args) >= 2 && args[1] == cmdGitCommonDir {
				return filepath.Join(repoDir, ".git"), nil
			}
			if len(args) >= 2 && args[1] == cmdShowToplevel {
				return repoDir, nil
			}
			return worktreeOut, nil
		},
		runInDir: noopRunInDir,
	}
	restore := overrideNewRunner(r)
	defer restore()

	cmd, _ := newTestCmd()
	cmd.Flags().String(flagAs, "", "")
	cmd.Flags().Bool(flagSkipDeps, false, "")
	cmd.Flags().Bool(flagSkipHooks, false, "")
	cmd.SetContext(config.WithConfig(context.Background(), cfg))

	err := duplicateCmd.RunE(cmd, []string{branchMain})
	if err == nil {
		t.Fatal("expected error for duplicating default branch")
	}
	if !strings.Contains(err.Error(), "cannot duplicate") {
		t.Errorf("error = %q, want 'cannot duplicate'", err.Error())
	}
}

func TestDuplicateAutoSuffix(t *testing.T) {
	repoDir := t.TempDir()
	wtDir := filepath.Join(repoDir, "worktrees")
	_ = os.MkdirAll(wtDir, 0755)
	cfg := &config.Config{DefaultSource: branchMain, WorktreeDir: "worktrees"}

	worktreeOut := strings.Join([]string{
		wtPrefix + repoDir,
		headABC123,
		branchRefMain,
		"",
		wtFeatureLogin,
		headDEF456,
		branchRefFeatureLogin,
		"",
	}, "\n")

	rollbackCalled := false
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			if len(args) >= 2 && args[0] == "worktree" && args[1] == cmdRemove {
				rollbackCalled = true
			}
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
	cmd.Flags().String(flagAs, "", "")
	cmd.Flags().Bool(flagSkipDeps, false, "")
	cmd.Flags().Bool(flagSkipHooks, false, "")
	_ = cmd.Flags().Set(flagSkipDeps, "true")
	_ = cmd.Flags().Set(flagSkipHooks, "true")
	cmd.SetContext(config.WithConfig(context.Background(), cfg))

	err := duplicateCmd.RunE(cmd, []string{"login"})
	if err != nil {
		t.Fatalf("duplicateCmd.RunE: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "Duplicated worktree") {
		t.Errorf("output = %q, want 'Duplicated worktree'", out)
	}
	if !strings.Contains(out, "login-1") {
		t.Errorf("output = %q, want auto-suffix 'login-1'", out)
	}
	if rollbackCalled {
		t.Fatal("successful duplicate must not roll back")
	}
}

func TestRollbackFailedDuplicateCleanupSucceeds(t *testing.T) {
	setupErr := errors.New("setup failed")
	cleanupContextLive := false
	r := &mockRunner{
		runContext: func(ctx context.Context, args ...string) (string, error) {
			if len(args) >= 2 && args[0] == "worktree" && args[1] == cmdRemove {
				cleanupContextLive = ctx.Err() == nil
			}
			return "", nil
		},
		runInDir: noopRunInDir,
	}

	err := rollbackFailedDuplicate(r, filepath.Join(t.TempDir(), "worktree"), duplicateRollbackBranch, "login-copy", setupErr, nil)
	assertErrorsAre(t, err, setupErr)
	if !cleanupContextLive {
		t.Error("cleanup context was cancelled, want live context")
	}
	assertErrorContains(t, err, "Rollback completed", duplicateRollbackBranch)
}

func TestRollbackFailedDuplicateWorktreeRemovalFails(t *testing.T) {
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
			if len(args) >= 2 && args[0] == "worktree" && args[1] == cmdRemove {
				return "", removeErr
			}
			if len(args) >= 1 && args[0] == cmdBranch {
				t.Fatal("branch deletion must not run after worktree removal fails")
			}
			return "", nil
		},
		runInDir: noopRunInDir,
	}

	err := rollbackFailedDuplicate(r, wtPath, duplicateRollbackBranch, "login-copy", setupErr, nil)
	assertErrorsAre(t, err, setupErr, removeErr)
	assertErrorContains(t, err, "Rollback failed", wtPath, "branch preserved")
}

func TestRollbackFailedDuplicateBranchDeletionFails(t *testing.T) {
	setupErr := errors.New("setup failed")
	branchErr := errors.New("branch delete failed")
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			if len(args) >= 1 && args[0] == cmdBranch {
				return "", branchErr
			}
			if len(args) >= 1 && args[0] == cmdRevParse {
				return "", nil
			}
			return "", nil
		},
		runInDir: noopRunInDir,
	}

	err := rollbackFailedDuplicate(r, filepath.Join(t.TempDir(), "worktree"), duplicateRollbackBranch, "login-copy", setupErr, nil)
	assertErrorsAre(t, err, setupErr, branchErr)
	assertErrorContains(t, err, "worktree removed", duplicateRollbackBranch, "failed to delete branch")
}

func TestRollbackFailedDuplicateLeavesDirectoryAndBranch(t *testing.T) {
	setupErr := errors.New("setup failed")
	branchErr := errors.New("branch delete failed")
	wtPath := filepath.Join(t.TempDir(), "$(unexpected)")
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			if len(args) >= 2 && args[0] == "worktree" && args[1] == cmdRemove {
				return "", errors.New("remove failed")
			}
			if len(args) >= 1 && args[0] == cmdBranch {
				return "", branchErr
			}
			if len(args) >= 1 && args[0] == cmdRevParse {
				return "", nil
			}
			return "", nil
		},
		runInDir: noopRunInDir,
	}

	err := rollbackFailedDuplicate(r, wtPath, duplicateRollbackBranch, "login-copy", setupErr, nil)
	assertErrorsAre(t, err, setupErr, branchErr)
	assertErrorContains(t, err, "directory remains", wtPath, duplicateRollbackBranch)
	if strings.Contains(err.Error(), "rm -rf --") {
		t.Errorf("error = %q, must not render a shell command", err)
	}
}

func TestRollbackFailedDuplicateFailedPruneLeavesDirectory(t *testing.T) {
	setupErr := errors.New("setup failed")
	pruneErr := errors.New("prune failed")
	wtPath := filepath.Join(t.TempDir(), "worktree")
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			if len(args) >= 2 && args[0] == "worktree" && args[1] == cmdRemove {
				return "", errors.New("remove failed")
			}
			if len(args) >= 2 && args[0] == "worktree" && args[1] == "prune" {
				return "", pruneErr
			}
			if len(args) >= 1 && args[0] == cmdBranch {
				t.Fatal("branch deletion must not run after failed prune")
			}
			return "", nil
		},
		runInDir: noopRunInDir,
	}

	err := rollbackFailedDuplicate(r, wtPath, duplicateRollbackBranch, "login-copy", setupErr, nil)
	assertErrorsAre(t, err, setupErr, pruneErr)
	assertErrorContains(t, err, "Rollback failed", "Directory remains", wtPath, "branch preserved")
}

func assertErrorsAre(t *testing.T, err error, wants ...error) {
	t.Helper()
	for _, want := range wants {
		if !errors.Is(err, want) {
			t.Errorf("errors.Is(err, %v) = false, err = %v", want, err)
		}
	}
}

func assertErrorContains(t *testing.T, err error, wants ...string) {
	t.Helper()
	for _, want := range wants {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error = %q, want substring %q", err, want)
		}
	}
}

func TestDuplicateWithAs(t *testing.T) {
	repoDir := t.TempDir()
	wtDir := filepath.Join(repoDir, "worktrees")
	_ = os.MkdirAll(wtDir, 0755)
	cfg := &config.Config{DefaultSource: branchMain, WorktreeDir: "worktrees"}

	worktreeOut := strings.Join([]string{
		wtPrefix + repoDir,
		headABC123,
		branchRefMain,
		"",
		wtFeatureLogin,
		headDEF456,
		branchRefFeatureLogin,
		"",
	}, "\n")

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
	cmd.Flags().String(flagAs, "", "")
	cmd.Flags().Bool(flagSkipDeps, false, "")
	cmd.Flags().Bool(flagSkipHooks, false, "")
	_ = cmd.Flags().Set(flagAs, "my-copy")
	_ = cmd.Flags().Set(flagSkipDeps, "true")
	_ = cmd.Flags().Set(flagSkipHooks, "true")
	cmd.SetContext(config.WithConfig(context.Background(), cfg))

	err := duplicateCmd.RunE(cmd, []string{"login"})
	if err != nil {
		t.Fatalf("duplicateCmd.RunE: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "my-copy") {
		t.Errorf("output = %q, want 'my-copy'", out)
	}
}

func TestDuplicateWithAsRetargetService(t *testing.T) {
	repoDir := t.TempDir()
	wtDir := filepath.Join(repoDir, "worktrees")
	_ = os.MkdirAll(wtDir, 0755)
	if err := os.Mkdir(filepath.Join(repoDir, "payments"), 0755); err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{DefaultSource: branchMain, WorktreeDir: "worktrees"}

	worktreeOut := strings.Join([]string{
		wtPrefix + repoDir,
		headABC123,
		branchRefMain,
		"",
		wtFeatureLogin,
		headDEF456,
		branchRefFeatureLogin,
		"",
	}, "\n")

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
	cmd.Flags().String(flagAs, "", "")
	cmd.Flags().Bool(flagSkipDeps, false, "")
	cmd.Flags().Bool(flagSkipHooks, false, "")
	_ = cmd.Flags().Set(flagAs, "payments/newname")
	_ = cmd.Flags().Set(flagSkipDeps, "true")
	_ = cmd.Flags().Set(flagSkipHooks, "true")
	cmd.SetContext(config.WithConfig(context.Background(), cfg))

	err := duplicateCmd.RunE(cmd, []string{"login"})
	if err != nil {
		t.Fatalf("duplicateCmd.RunE: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "payments/feature/newname") {
		t.Errorf("output = %q, want branch 'payments/feature/newname'", out)
	}
}

func TestDuplicateWithAsUnknownServiceErrors(t *testing.T) {
	repoDir := t.TempDir()
	wtDir := filepath.Join(repoDir, "worktrees")
	_ = os.MkdirAll(wtDir, 0755)
	cfg := &config.Config{DefaultSource: branchMain, WorktreeDir: "worktrees"}

	worktreeOut := strings.Join([]string{
		wtPrefix + repoDir,
		headABC123,
		branchRefMain,
		"",
		wtFeatureLogin,
		headDEF456,
		branchRefFeatureLogin,
		"",
	}, "\n")

	addWorktreeCalled := false
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
		runInDir: func(_ string, args ...string) (string, error) {
			if len(args) >= 2 && args[0] == cmdWorktreeTest && args[1] == gitSubcmdWorktreeAdd {
				addWorktreeCalled = true
			}
			return "", nil
		},
	}
	restore := overrideNewRunner(r)
	defer restore()

	cmd, _ := newTestCmd()
	cmd.Flags().String(flagAs, "", "")
	cmd.Flags().Bool(flagSkipDeps, false, "")
	cmd.Flags().Bool(flagSkipHooks, false, "")
	_ = cmd.Flags().Set(flagAs, "ghost/newname")
	cmd.SetContext(config.WithConfig(context.Background(), cfg))

	err := duplicateCmd.RunE(cmd, []string{"login"})
	if err == nil {
		t.Fatal("expected error for unknown service in --as")
	}
	if !strings.Contains(err.Error(), `service "ghost" not found`) {
		t.Errorf("error = %q, want it to mention `service \"ghost\" not found`", err.Error())
	}
	if addWorktreeCalled {
		t.Error("git worktree add must not be called for unknown service")
	}
}

func TestDuplicateRejectsUnsafeAs(t *testing.T) {
	cases := []struct {
		name string
		as   string
	}{
		{name: "leading dash", as: "-x"},
		{name: "leading double-dot", as: "..branch"},
		{name: "path traversal", as: "../bad"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repoDir := t.TempDir()
			wtDir := filepath.Join(repoDir, "worktrees")
			_ = os.MkdirAll(wtDir, 0755)
			cfg := &config.Config{DefaultSource: branchMain, WorktreeDir: "worktrees"}

			worktreeOut := strings.Join([]string{
				wtPrefix + repoDir,
				headABC123,
				branchRefMain,
				"",
				wtFeatureLogin,
				headDEF456,
				branchRefFeatureLogin,
				"",
			}, "\n")

			addWorktreeCalled := false
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
				runInDir: func(_ string, args ...string) (string, error) {
					if len(args) >= 2 && args[0] == cmdWorktreeTest && args[1] == "add" {
						addWorktreeCalled = true
					}
					return "", nil
				},
			}
			restore := overrideNewRunner(r)
			defer restore()

			cmd, _ := newTestCmd()
			cmd.Flags().String(flagAs, "", "")
			cmd.Flags().Bool(flagSkipDeps, false, "")
			cmd.Flags().Bool(flagSkipHooks, false, "")
			_ = cmd.Flags().Set(flagAs, tc.as)
			cmd.SetContext(config.WithConfig(context.Background(), cfg))

			err := duplicateCmd.RunE(cmd, []string{"login"})
			if err == nil {
				t.Fatalf("duplicateCmd.RunE(--as %q) = nil error, want rejection", tc.as)
			}
			if !strings.Contains(err.Error(), "unsafe git ref name") && !strings.Contains(err.Error(), "invalid") {
				t.Errorf("error = %q, want it to mention 'unsafe git ref name' or 'invalid'", err.Error())
			}
			if addWorktreeCalled {
				t.Errorf("--as %q: git worktree add must not be called for unsafe input", tc.as)
			}
		})
	}
}

func TestDuplicateBranchAlreadyExists(t *testing.T) {
	repoDir := t.TempDir()
	wtDir := filepath.Join(repoDir, "worktrees")
	_ = os.MkdirAll(wtDir, 0755)
	cfg := &config.Config{DefaultSource: branchMain, WorktreeDir: "worktrees"}

	worktreeOut := strings.Join([]string{
		wtPrefix + repoDir,
		headABC123,
		branchRefMain,
		"",
		wtFeatureLogin,
		headDEF456,
		branchRefFeatureLogin,
		"",
	}, "\n")

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
	cmd.Flags().String(flagAs, "", "")
	cmd.Flags().Bool(flagSkipDeps, false, "")
	cmd.Flags().Bool(flagSkipHooks, false, "")
	_ = cmd.Flags().Set(flagAs, "existing")
	_ = cmd.Flags().Set(flagSkipDeps, "true")
	cmd.SetContext(config.WithConfig(context.Background(), cfg))

	err := duplicateCmd.RunE(cmd, []string{"login"})
	if err == nil {
		t.Fatal("expected error for existing branch")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Errorf("error = %q, want 'already exists'", err.Error())
	}
}

func TestDuplicateWorktreePathAlreadyExists(t *testing.T) {
	repoDir := t.TempDir()
	wtDir := filepath.Join(repoDir, "worktrees")
	_ = os.MkdirAll(wtDir, 0755)
	cfg := &config.Config{DefaultSource: branchMain, WorktreeDir: "worktrees"}

	// Create the target worktree path on disk (branch doesn't exist, but directory does)
	destBranch := resolver.FullBranchName("", "feature/", "orphaned")
	destPath := resolver.WorktreePath(wtDir, destBranch)
	_ = os.MkdirAll(destPath, 0755)

	worktreeOut := strings.Join([]string{
		wtPrefix + repoDir,
		headABC123,
		branchRefMain,
		"",
		wtFeatureLogin,
		headDEF456,
		branchRefFeatureLogin,
		"",
	}, "\n")

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

	cmd, _ := newTestCmd()
	cmd.Flags().String(flagAs, "", "")
	cmd.Flags().Bool(flagSkipDeps, false, "")
	cmd.Flags().Bool(flagSkipHooks, false, "")
	_ = cmd.Flags().Set(flagAs, "orphaned")
	cmd.SetContext(config.WithConfig(context.Background(), cfg))

	err := duplicateCmd.RunE(cmd, []string{"login"})
	if err == nil {
		t.Fatal("expected error for existing worktree path")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Errorf("error = %q, want 'already exists'", err.Error())
	}
}

func TestDuplicateDryRun(t *testing.T) {
	repoDir := t.TempDir()
	wtDir := filepath.Join(repoDir, "worktrees")
	_ = os.MkdirAll(wtDir, 0755)
	cfg := &config.Config{DefaultSource: branchMain, WorktreeDir: "worktrees"}

	worktreeOut := strings.Join([]string{
		wtPrefix + repoDir,
		headABC123,
		branchRefMain,
		"",
		wtFeatureLogin,
		headDEF456,
		branchRefFeatureLogin,
		"",
	}, "\n")

	addWorktreeCalled := false
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
		runInDir: func(_ string, args ...string) (string, error) {
			if len(args) >= 2 && args[0] == cmdWorktreeTest && args[1] == "add" {
				addWorktreeCalled = true
			}
			return "", nil
		},
	}
	restore := overrideNewRunner(r)
	defer restore()

	cmd, buf := newTestCmd()
	cmd.Flags().String(flagAs, "", "")
	cmd.Flags().Bool(flagSkipDeps, false, "")
	cmd.Flags().Bool(flagSkipHooks, false, "")
	cmd.Flags().Bool(flagDryRun, false, "")
	_ = cmd.Flags().Set(flagDryRun, "true")
	cmd.SetContext(config.WithConfig(context.Background(), cfg))

	if err := duplicateCmd.RunE(cmd, []string{"login"}); err != nil {
		t.Fatalf("duplicateCmd.RunE: %v", err)
	}
	if addWorktreeCalled {
		t.Error("git worktree add must not be called in dry-run mode")
	}
	out := buf.String()
	if !strings.Contains(out, "[dry-run]") {
		t.Errorf("output = %q, want '[dry-run]' prefix", out)
	}
	if strings.Contains(out, "Duplicated worktree") {
		t.Errorf("output = %q, must not contain 'Duplicated worktree' in dry-run", out)
	}
}

func TestDuplicateWorktreeNotFound(t *testing.T) {
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
			return worktreeOut, nil
		},
		runInDir: noopRunInDir,
	}
	restore := overrideNewRunner(r)
	defer restore()

	cmd, _ := newTestCmd()
	cmd.Flags().String(flagAs, "", "")
	cmd.Flags().Bool(flagSkipDeps, false, "")
	cmd.Flags().Bool(flagSkipHooks, false, "")
	cmd.SetContext(config.WithConfig(context.Background(), cfg))

	err := duplicateCmd.RunE(cmd, []string{"nonexistent"})
	if err == nil {
		t.Fatal("expected error for missing worktree")
	}
}

func TestDuplicateOrphanedPrefixHardErrors(t *testing.T) {
	repoDir := t.TempDir()

	r := &mockRunner{
		run: func(args ...string) (string, error) {
			if len(args) >= 2 && args[1] == cmdGitCommonDir {
				return filepath.Join(repoDir, ".git"), nil
			}
			if len(args) >= 2 && args[1] == cmdShowToplevel {
				return repoDir, nil
			}
			return orphanedProjWorktreeOut, nil
		},
		runInDir: noopRunInDir,
	}
	restore := overrideNewRunner(r)
	defer restore()

	cmd, _ := newTestCmd()
	cmd.Flags().String(flagAs, "", "")
	cmd.Flags().Bool(flagSkipDeps, false, "")
	cmd.Flags().Bool(flagSkipHooks, false, "")
	cmd.SetContext(config.WithConfig(context.Background(), orphanedRemoveConfig()))

	err := duplicateCmd.RunE(cmd, []string{"PROJ-123"})
	if err == nil {
		t.Fatal("expected orphan-guard error, got nil")
	}
	if !strings.Contains(err.Error(), "re-add the prefix") {
		t.Errorf("error = %q, want it to mention re-adding the prefix", err.Error())
	}
}

func TestDuplicateAutoSuffixSkipsExistingPath(t *testing.T) {
	repoDir := t.TempDir()
	wtDir := filepath.Join(repoDir, "worktrees")
	_ = os.MkdirAll(wtDir, 0755)
	cfg := &config.Config{DefaultSource: branchMain, WorktreeDir: "worktrees"}

	// Stray directory occupies the -1 path although no such branch exists.
	strayBranch := resolver.FullBranchName("", "feature/", "login-1")
	_ = os.MkdirAll(resolver.WorktreePath(wtDir, strayBranch), 0755)

	worktreeOut := strings.Join([]string{
		wtPrefix + repoDir,
		headABC123,
		branchRefMain,
		"",
		wtFeatureLogin,
		headDEF456,
		branchRefFeatureLogin,
		"",
	}, "\n")

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
	cmd.Flags().String(flagAs, "", "")
	cmd.Flags().Bool(flagSkipDeps, false, "")
	cmd.Flags().Bool(flagSkipHooks, false, "")
	_ = cmd.Flags().Set(flagSkipDeps, "true")
	_ = cmd.Flags().Set(flagSkipHooks, "true")
	cmd.SetContext(config.WithConfig(context.Background(), cfg))

	if err := duplicateCmd.RunE(cmd, []string{"login"}); err != nil {
		t.Fatalf("duplicateCmd.RunE: %v", err)
	}
	if out := buf.String(); !strings.Contains(out, "login-2") {
		t.Errorf("output = %q, want auto-suffix 'login-2'", out)
	}
}

func TestNextDuplicateTask(t *testing.T) {
	tests := []struct {
		name     string
		svc      string
		occupied []string // suffixed tasks with a directory on disk
		dangling []string // suffixed tasks with a dangling symlink on disk
		branches []string // suffixed tasks whose branch exists
		want     string
	}{
		{name: "no collision", want: "login-1"},
		{name: "path collision", occupied: []string{"login-1"}, want: "login-2"},
		{name: "dangling symlink collision", dangling: []string{"login-1"}, want: "login-2"},
		{name: "service scoped path collision", svc: "auth-api", occupied: []string{"login-1"}, want: "login-2"},
		{name: "branch collision", branches: []string{"login-1"}, want: "login-2"},
		{name: "mixed collisions", occupied: []string{"login-1"}, branches: []string{"login-2"}, want: "login-3"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wtDir := t.TempDir()
			for _, o := range tt.occupied {
				b := resolver.FullBranchName(tt.svc, "feature/", o)
				_ = os.MkdirAll(resolver.WorktreePath(wtDir, b), 0755)
			}
			for _, d := range tt.dangling {
				b := resolver.FullBranchName(tt.svc, "feature/", d)
				p := resolver.WorktreePath(wtDir, b)
				if err := os.Symlink(filepath.Join(wtDir, "missing-target"), p); err != nil {
					t.Skipf("symlinks unavailable: %v", err)
				}
			}
			r := &mockRunner{
				run: func(args ...string) (string, error) {
					for _, b := range tt.branches {
						if strings.HasSuffix(args[len(args)-1], "/"+b) {
							return "", nil
						}
					}
					return "", errGitFailed
				},
				runInDir: noopRunInDir,
			}
			got, err := nextDuplicateTask(context.Background(), r, "login", tt.svc, "feature/", wtDir)
			if err != nil {
				t.Fatalf("nextDuplicateTask: %v", err)
			}
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestNextDuplicateTaskExhausted(t *testing.T) {
	r := &mockRunner{
		run:      func(args ...string) (string, error) { return "", nil }, // every branch exists
		runInDir: noopRunInDir,
	}
	_, err := nextDuplicateTask(context.Background(), r, "login", "", "feature/", t.TempDir())
	if err == nil || !strings.Contains(err.Error(), "could not find available suffix") {
		t.Fatalf("err = %v, want 'could not find available suffix'", err)
	}
}

func TestNextDuplicateTaskPathCheckError(t *testing.T) {
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("permission-based test not reliable here")
	}
	wtDir := t.TempDir()
	if err := os.Chmod(wtDir, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(wtDir, 0755) })

	r := &mockRunner{
		run:      func(args ...string) (string, error) { return "", errGitFailed },
		runInDir: noopRunInDir,
	}
	_, err := nextDuplicateTask(context.Background(), r, "login", "", "feature/", filepath.Join(wtDir, "sub"))
	if err == nil || !strings.Contains(err.Error(), "check worktree path") {
		t.Fatalf("err = %v, want 'check worktree path'", err)
	}
}

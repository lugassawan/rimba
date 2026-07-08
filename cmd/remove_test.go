package cmd

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/lugassawan/rimba/internal/config"
)

const (
	removeWorktreeOut = "worktree /repo\nHEAD abc123\nbranch refs/heads/main\n\nworktree /wt/feature-login\nHEAD def456\nbranch refs/heads/feature/login\n"
)

func TestRemoveSuccess(t *testing.T) {
	r := &mockRunner{
		run:      func(_ ...string) (string, error) { return removeWorktreeOut, nil },
		runInDir: noopRunInDir,
	}
	restore := overrideNewRunner(r)
	defer restore()

	cmd, buf := newTestCmd()
	cmd.SetContext(config.WithConfig(context.Background(), &config.Config{}))
	cmd.Flags().Bool(flagKeepBranch, false, "")
	cmd.Flags().Bool(flagForce, false, "")

	err := removeCmd.RunE(cmd, []string{"login"})
	if err != nil {
		t.Fatalf("removeCmd.RunE: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "Removed worktree") {
		t.Errorf("output = %q, want 'Removed worktree'", out)
	}
	if !strings.Contains(out, "Deleted branch") {
		t.Errorf("output = %q, want 'Deleted branch'", out)
	}
}

func TestRemoveKeepBranch(t *testing.T) {
	r := &mockRunner{
		run:      func(_ ...string) (string, error) { return removeWorktreeOut, nil },
		runInDir: noopRunInDir,
	}
	restore := overrideNewRunner(r)
	defer restore()

	cmd, buf := newTestCmd()
	cmd.SetContext(config.WithConfig(context.Background(), &config.Config{}))
	cmd.Flags().Bool(flagKeepBranch, false, "")
	cmd.Flags().Bool(flagForce, false, "")
	_ = cmd.Flags().Set(flagKeepBranch, "true")

	err := removeCmd.RunE(cmd, []string{"login"})
	if err != nil {
		t.Fatalf("removeCmd.RunE: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "Removed worktree") {
		t.Errorf("output = %q, want 'Removed worktree'", out)
	}
	if strings.Contains(out, "Deleted branch") {
		t.Errorf("output = %q, should not contain 'Deleted branch' with --keep-branch", out)
	}
}

// orphanedRemoveConfig returns a config where "TASK-" is the only configured
// custom prefix, so a "PROJ-*" branch (created under a prefix that used to be
// configured but no longer is) is orphaned while HasCustom() stays true.
func orphanedRemoveConfig() *config.Config {
	return &config.Config{
		DefaultSource: branchMain,
		Resolver: &config.ResolverConfig{
			Prefix: []config.PrefixEntry{{Prefix: "TASK-"}},
		},
	}
}

func TestRemoveOrphanedPrefixHardErrors(t *testing.T) {
	worktreeOut := orphanedProjWorktreeOut
	r := &mockRunner{
		run:      func(_ ...string) (string, error) { return worktreeOut, nil },
		runInDir: noopRunInDir,
	}
	restore := overrideNewRunner(r)
	defer restore()

	cmd, _ := newTestCmd()
	cmd.SetContext(config.WithConfig(context.Background(), orphanedRemoveConfig()))
	cmd.Flags().Bool(flagKeepBranch, false, "")
	cmd.Flags().Bool(flagForce, false, "")

	err := removeCmd.RunE(cmd, []string{"PROJ-123"})
	if err == nil {
		t.Fatal("expected orphan-guard error, got nil")
	}
	if !strings.Contains(err.Error(), "re-add the prefix") {
		t.Errorf("error = %q, want it to mention re-adding the prefix", err.Error())
	}
}

func TestRemoveOrphanedPrefixForceBypasses(t *testing.T) {
	worktreeOut := orphanedProjWorktreeOut
	r := &mockRunner{
		run:      func(_ ...string) (string, error) { return worktreeOut, nil },
		runInDir: noopRunInDir,
	}
	restore := overrideNewRunner(r)
	defer restore()

	cmd, buf := newTestCmd()
	cmd.SetContext(config.WithConfig(context.Background(), orphanedRemoveConfig()))
	cmd.Flags().Bool(flagKeepBranch, false, "")
	cmd.Flags().Bool(flagForce, false, "")
	_ = cmd.Flags().Set(flagForce, "true")

	err := removeCmd.RunE(cmd, []string{"PROJ-123"})
	if err != nil {
		t.Fatalf("removeCmd.RunE with --force: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "Removed worktree") {
		t.Errorf("output = %q, want 'Removed worktree'", out)
	}
}

func TestRemoveWorktreeNotFound(t *testing.T) {
	r := &mockRunner{
		run:      func(_ ...string) (string, error) { return removeWorktreeOut, nil },
		runInDir: noopRunInDir,
	}
	restore := overrideNewRunner(r)
	defer restore()

	cmd, _ := newTestCmd()
	cmd.SetContext(config.WithConfig(context.Background(), &config.Config{}))
	cmd.Flags().Bool(flagKeepBranch, false, "")
	cmd.Flags().Bool(flagForce, false, "")

	err := removeCmd.RunE(cmd, []string{"nonexistent"})
	if err == nil {
		t.Fatal("expected error for missing worktree")
	}
}

func TestRemoveWorktreeFails(t *testing.T) {
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			if len(args) >= 2 && args[0] == cmdWorktreeTest && args[1] == "remove" {
				return "", errors.New("locked")
			}
			return removeWorktreeOut, nil
		},
		runInDir: noopRunInDir,
	}
	restore := overrideNewRunner(r)
	defer restore()

	cmd, _ := newTestCmd()
	cmd.SetContext(config.WithConfig(context.Background(), &config.Config{}))
	cmd.Flags().Bool(flagKeepBranch, false, "")
	cmd.Flags().Bool(flagForce, false, "")

	err := removeCmd.RunE(cmd, []string{"login"})
	if err == nil {
		t.Fatal("expected error from worktree remove failure")
	}
}

func TestRemoveDryRun(t *testing.T) {
	r := &mockRunner{
		run:      func(_ ...string) (string, error) { return removeWorktreeOut, nil },
		runInDir: noopRunInDir,
	}
	restore := overrideNewRunner(r)
	defer restore()

	cmd, buf := newTestCmd()
	cmd.SetContext(config.WithConfig(context.Background(), &config.Config{}))
	cmd.Flags().Bool(flagKeepBranch, false, "")
	cmd.Flags().Bool(flagForce, false, "")
	cmd.Flags().Bool(flagDryRun, false, "")
	_ = cmd.Flags().Set(flagDryRun, "true")

	if err := removeCmd.RunE(cmd, []string{"login"}); err != nil {
		t.Fatalf("removeCmd.RunE: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "[dry-run]") {
		t.Errorf("output = %q, want '[dry-run]' prefix", out)
	}
	if strings.Contains(out, "Removed worktree") {
		t.Errorf("output = %q, must not contain 'Removed worktree' in dry-run mode", out)
	}
}

func TestRemoveBranchDeleteFails(t *testing.T) {
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			if len(args) >= 1 && args[0] == cmdBranch {
				return "", errors.New("branch error")
			}
			return removeWorktreeOut, nil
		},
		runInDir: noopRunInDir,
	}
	restore := overrideNewRunner(r)
	defer restore()

	cmd, buf := newTestCmd()
	cmd.SetContext(config.WithConfig(context.Background(), &config.Config{}))
	cmd.Flags().Bool(flagKeepBranch, false, "")
	cmd.Flags().Bool(flagForce, false, "")

	err := removeCmd.RunE(cmd, []string{"login"})
	if err != nil {
		t.Fatalf("expected no error (branch delete failure is non-fatal), got: %v", err)
	}
	if !strings.Contains(buf.String(), "failed to delete branch") {
		t.Errorf("output = %q, want branch delete failure message", buf.String())
	}
}

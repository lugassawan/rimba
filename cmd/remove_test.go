package cmd

import (
	"errors"
	"strings"
	"testing"
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

func TestRemoveWorktreeNotFound(t *testing.T) {
	r := &mockRunner{
		run:      func(_ ...string) (string, error) { return removeWorktreeOut, nil },
		runInDir: noopRunInDir,
	}
	restore := overrideNewRunner(r)
	defer restore()

	cmd, _ := newTestCmd()
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
	cmd.Flags().Bool(flagKeepBranch, false, "")
	cmd.Flags().Bool(flagForce, false, "")

	err := removeCmd.RunE(cmd, []string{"login"})
	if err == nil {
		t.Fatal("expected error from worktree remove failure")
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

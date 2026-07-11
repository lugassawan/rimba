package cmd

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lugassawan/rimba/internal/config"
)

const wantRemoveCommand = "remove"

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

func TestRemovePrunablePathHealsAndRemoves(t *testing.T) {
	prunableWorktreeOut := "worktree /repo\nHEAD abc123\nbranch refs/heads/main\n\n" +
		"worktree /wt/feature-login\nHEAD def456\nbranch refs/heads/feature/login\nprunable gitdir file points to non-existent location\n"

	var repairInvoked, removeInvoked, pruneInvoked bool
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			cmd := strings.Join(args, " ")
			switch {
			case strings.Contains(cmd, "worktree repair"):
				repairInvoked = true
			case strings.Contains(cmd, "worktree remove"):
				removeInvoked = true
			case strings.Contains(cmd, "worktree prune"):
				pruneInvoked = true
			}
			if len(args) > 0 && args[0] == cmdWorktreeTest && args[1] == cmdList {
				return prunableWorktreeOut, nil
			}
			return "", nil
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
		t.Fatalf("removeCmd.RunE: %v", err)
	}
	if !repairInvoked {
		t.Error("expected 'git worktree repair' to be invoked for a prunable worktree")
	}
	if !removeInvoked {
		t.Error("expected 'git worktree remove' to be invoked after repair")
	}
	if pruneInvoked {
		t.Error("expected 'git worktree prune' NOT to be invoked when repair+remove succeed")
	}
	out := buf.String()
	if !strings.Contains(out, "Removed worktree") {
		t.Errorf("output = %q, want 'Removed worktree' — repair+remove fully cleared the directory", out)
	}
	if strings.Contains(out, "Cleared stale worktree registration") {
		t.Errorf("output = %q, want NOT 'Cleared stale worktree registration' once repair+remove succeed", out)
	}
}

func TestRemovePrunablePathFallbackMessage(t *testing.T) {
	prunableWorktreeOut := "worktree /repo\nHEAD abc123\nbranch refs/heads/main\n\n" +
		"worktree /wt/feature-login\nHEAD def456\nbranch refs/heads/feature/login\nprunable gitdir file points to non-existent location\n"

	r := &mockRunner{
		run: func(args ...string) (string, error) {
			if len(args) > 0 && args[0] == cmdWorktreeTest && args[1] == cmdList {
				return prunableWorktreeOut, nil
			}
			if len(args) > 1 && args[0] == "worktree" && args[1] == "remove" {
				return "", errors.New("remove failed")
			}
			return "", nil
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
		t.Fatalf("removeCmd.RunE: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "Cleared stale worktree registration") {
		t.Errorf("output = %q, want a distinct prunable-recovery message, not 'Removed worktree' (git worktree prune leaves the directory on disk)", out)
	}
	if strings.Contains(out, "Removed worktree") {
		t.Errorf("output = %q, want NOT 'Removed worktree' for the prunable-recovery path", out)
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

// orphanedRemoveConfig configures only "TASK-", so a "PROJ-*" branch is
// orphaned while HasCustom() stays true.
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
	// A real .git file makes this a genuine (non-orphaned) failure, so it
	// short-circuits instead of routing through the heal-and-retry path.
	wtPath := t.TempDir()
	if err := os.WriteFile(filepath.Join(wtPath, ".git"), []byte("gitdir: /somewhere/.git/worktrees/login\n"), 0o644); err != nil {
		t.Fatalf("failed to create .git fixture: %v", err)
	}
	worktreeOut := "worktree /repo\nHEAD abc123\nbranch refs/heads/main\n\nworktree " + wtPath + "\nHEAD def456\nbranch refs/heads/feature/login\n"

	r := &mockRunner{
		run: func(args ...string) (string, error) {
			if len(args) >= 2 && args[0] == cmdWorktreeTest && args[1] == "remove" {
				return "", errors.New("locked")
			}
			return worktreeOut, nil
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

func TestRemoveJSON(t *testing.T) {
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
	_ = cmd.Flags().Set(flagJSON, "true")

	if err := removeCmd.RunE(cmd, []string{"login"}); err != nil {
		t.Fatalf("removeCmd.RunE: %v", err)
	}

	env, data := decodeAddEnvelope(t, buf.Bytes())
	if env.Command != wantRemoveCommand {
		t.Errorf("command = %q, want %q", env.Command, wantRemoveCommand)
	}
	if data["worktree_removed"] != true {
		t.Errorf("worktree_removed = %v, want true", data["worktree_removed"])
	}
	if data["branch_deleted"] != true {
		t.Errorf("branch_deleted = %v, want true", data["branch_deleted"])
	}
	if data["dry_run"] != false {
		t.Errorf("dry_run = %v, want false", data["dry_run"])
	}
}

func TestRemoveDryRunJSON(t *testing.T) {
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
	_ = cmd.Flags().Set(flagJSON, "true")

	if err := removeCmd.RunE(cmd, []string{"login"}); err != nil {
		t.Fatalf("removeCmd.RunE: %v", err)
	}

	env, data := decodeAddEnvelope(t, buf.Bytes())
	if env.Command != wantRemoveCommand {
		t.Errorf("command = %q, want %q", env.Command, wantRemoveCommand)
	}
	if data["dry_run"] != true {
		t.Errorf("dry_run = %v, want true", data["dry_run"])
	}
	if data["worktree_removed"] != false {
		t.Errorf("worktree_removed = %v, want false", data["worktree_removed"])
	}
	if data["branch_deleted"] != false {
		t.Errorf("branch_deleted = %v, want false", data["branch_deleted"])
	}
}

func TestRemoveBranchErrorJSON(t *testing.T) {
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
	_ = cmd.Flags().Set(flagJSON, "true")

	err := removeCmd.RunE(cmd, []string{"login"})
	if err != nil {
		t.Fatalf("expected no error (branch delete failure is non-fatal), got: %v", err)
	}

	_, data := decodeAddEnvelope(t, buf.Bytes())
	branchErr, _ := data["branch_error"].(string)
	if branchErr == "" {
		t.Errorf("branch_error = %q, want non-empty", branchErr)
	}
}

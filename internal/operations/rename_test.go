package operations

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/lugassawan/rimba/internal/resolver"
)

const (
	cmdRevParse     = "rev-parse"
	cmdWorktree     = "worktree"
	cmdMove         = "move"
	wtDir           = "/worktrees"
	branchAuth      = "feature/auth"
	errMoveFailed   = "move failed"
	errRenameFailed = "rename failed"
)

func TestRenameWorktreeSuccess(t *testing.T) {
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			if len(args) >= 1 && args[0] == cmdRevParse {
				return "", errGitFailed // BranchExists returns false
			}
			return "", nil
		},
		runInDir: noopRunInDir,
	}

	wt := resolver.WorktreeInfo{Branch: branchFeature, Path: pathWtFeatureLogin}
	res, err := RenameWorktree(context.Background(), r, wt, "auth", wtDir, false)
	if err != nil {
		t.Fatalf("RenameWorktree: %v", err)
	}
	if res.OldBranch != branchFeature {
		t.Errorf("OldBranch = %q, want %q", res.OldBranch, branchFeature)
	}
	if res.NewBranch != branchAuth {
		t.Errorf("NewBranch = %q, want %q", res.NewBranch, branchAuth)
	}
	if res.OldPath != pathWtFeatureLogin {
		t.Errorf("OldPath = %q, want %q", res.OldPath, pathWtFeatureLogin)
	}
}

func TestRenameWorktreeBranchExists(t *testing.T) {
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			if len(args) >= 1 && args[0] == cmdRevParse {
				return "", nil // BranchExists returns true
			}
			return "", nil
		},
		runInDir: noopRunInDir,
	}

	wt := resolver.WorktreeInfo{Branch: branchFeature, Path: pathWtFeatureLogin}
	_, err := RenameWorktree(context.Background(), r, wt, "auth", wtDir, false)
	if err == nil {
		t.Fatal("expected error for existing branch")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Errorf("error = %q, want 'already exists'", err.Error())
	}
	if !strings.Contains(err.Error(), "To fix:") {
		t.Errorf("error = %q, want 'To fix:' hint", err.Error())
	}
	if !strings.Contains(err.Error(), "git branch -D") {
		t.Errorf("error = %q, want 'git branch -D' hint fragment", err.Error())
	}
}

func TestRenameWorktreeSameName(t *testing.T) {
	calls := 0
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			calls++
			return "", nil
		},
		runInDir: func(dir string, args ...string) (string, error) {
			calls++
			return "", nil
		},
	}

	wt := resolver.WorktreeInfo{Branch: branchFeature, Path: pathWtFeatureLogin}
	_, err := RenameWorktree(context.Background(), r, wt, "login", wtDir, false)
	if err == nil {
		t.Fatal("expected error for same-name rename")
	}
	if !strings.Contains(err.Error(), "same as the current name") {
		t.Errorf("error = %q, want same-name message", err.Error())
	}
	if strings.Contains(err.Error(), "git branch -D") {
		t.Errorf("error = %q, must not include destructive branch delete hint", err.Error())
	}
	if strings.Contains(err.Error(), "To fix:") {
		t.Errorf("error = %q, must not include a recovery hint", err.Error())
	}
	if calls != 0 {
		t.Fatalf("expected no git calls for same-name rename, got %d", calls)
	}
}

func TestRenameWorktreeMoveFails(t *testing.T) {
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			if len(args) >= 1 && args[0] == cmdRevParse {
				return "", errGitFailed
			}
			if len(args) >= 2 && args[0] == cmdWorktree && args[1] == cmdMove {
				return "", errors.New(errMoveFailed)
			}
			return "", nil
		},
		runInDir: noopRunInDir,
	}

	wt := resolver.WorktreeInfo{Branch: branchFeature, Path: pathWtFeatureLogin}
	_, err := RenameWorktree(context.Background(), r, wt, "auth", wtDir, false)
	if err == nil {
		t.Fatal("expected error from move failure")
	}
	if !strings.Contains(err.Error(), "To fix:") {
		t.Errorf("error = %q, want 'To fix:' hint", err.Error())
	}
	if !strings.Contains(err.Error(), "git worktree unlock") {
		t.Errorf("error = %q, want 'git worktree unlock' hint fragment", err.Error())
	}
}

func TestRenameWorktreeBranchRenameFails(t *testing.T) {
	moveCount := 0
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			if len(args) >= 1 && args[0] == cmdRevParse {
				return "", errGitFailed
			}
			if len(args) >= 2 && args[0] == cmdWorktree && args[1] == cmdMove {
				moveCount++
				return "", nil
			}
			if len(args) >= 2 && args[0] == "branch" && args[1] == "-m" {
				return "", errors.New(errRenameFailed)
			}
			return "", nil
		},
		runInDir: noopRunInDir,
	}

	wt := resolver.WorktreeInfo{Branch: branchFeature, Path: pathWtFeatureLogin}
	_, err := RenameWorktree(context.Background(), r, wt, "auth", wtDir, false)
	if err == nil {
		t.Fatal("expected error from branch rename failure")
	}
	if !strings.Contains(err.Error(), "failed to rename branch") {
		t.Errorf("error = %q, want 'failed to rename branch'", err.Error())
	}
	if !strings.Contains(err.Error(), "moved back") {
		t.Errorf("error = %q, want 'moved back' (rollback confirmation)", err.Error())
	}
	if !strings.Contains(err.Error(), "To fix:") {
		t.Errorf("error = %q, want 'To fix:' hint", err.Error())
	}
	if !strings.Contains(err.Error(), "git branch -m") {
		t.Errorf("error = %q, want 'git branch -m' hint fragment", err.Error())
	}
	if moveCount != 2 {
		t.Errorf("expected 2 worktree move calls (forward + rollback), got %d", moveCount)
	}
}

func TestRenameWorktreeBranchRenameFailsRollbackFails(t *testing.T) {
	moveCount := 0
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			if len(args) >= 1 && args[0] == cmdRevParse {
				return "", errGitFailed
			}
			if len(args) >= 2 && args[0] == cmdWorktree && args[1] == cmdMove {
				moveCount++
				if moveCount == 2 {
					return "", errors.New("rollback move failed")
				}
				return "", nil
			}
			if len(args) >= 2 && args[0] == "branch" && args[1] == "-m" {
				return "", errors.New(errRenameFailed)
			}
			return "", nil
		},
		runInDir: noopRunInDir,
	}

	wt := resolver.WorktreeInfo{Branch: branchFeature, Path: pathWtFeatureLogin}
	_, err := RenameWorktree(context.Background(), r, wt, "auth", wtDir, false)
	if err == nil {
		t.Fatal("expected error from branch rename + rollback failure")
	}
	if !strings.Contains(err.Error(), "failed to rename branch") {
		t.Errorf("error = %q, want 'failed to rename branch'", err.Error())
	}
	if !strings.Contains(err.Error(), "Rollback failed") {
		t.Errorf("error = %q, want 'Rollback failed'", err.Error())
	}
	if !strings.Contains(err.Error(), "To fix:") {
		t.Errorf("error = %q, want 'To fix:' hint", err.Error())
	}
	if !strings.Contains(err.Error(), "git worktree move") {
		t.Errorf("error = %q, want 'git worktree move' hint fragment", err.Error())
	}
}

func TestRenameWorktreeNoPrefixMatch(t *testing.T) {
	// Branch without a recognized prefix falls back to default prefix
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			if len(args) >= 1 && args[0] == cmdRevParse {
				return "", errGitFailed
			}
			return "", nil
		},
		runInDir: noopRunInDir,
	}

	wt := resolver.WorktreeInfo{Branch: "plain-branch", Path: "/wt/plain-branch"}
	res, err := RenameWorktree(context.Background(), r, wt, "new-task", wtDir, false)
	if err != nil {
		t.Fatalf("RenameWorktree: %v", err)
	}
	// Should use default prefix (feature/)
	if !strings.HasPrefix(res.NewBranch, "feature/") {
		t.Errorf("NewBranch = %q, want feature/ prefix", res.NewBranch)
	}
}

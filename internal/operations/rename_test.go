package operations

import (
	"errors"
	"strings"
	"testing"

	"github.com/lugassawan/rimba/internal/resolver"
)

const (
	cmdRevParse     = "rev-parse"
	cmdWorktree     = "worktree"
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
	res, err := RenameWorktree(r, wt, "auth", wtDir, false)
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
	_, err := RenameWorktree(r, wt, "auth", wtDir, false)
	if err == nil {
		t.Fatal("expected error for existing branch")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Errorf("error = %q, want 'already exists'", err.Error())
	}
}

func TestRenameWorktreeMoveFails(t *testing.T) {
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			if len(args) >= 1 && args[0] == cmdRevParse {
				return "", errGitFailed
			}
			if len(args) >= 2 && args[0] == cmdWorktree && args[1] == "move" {
				return "", errors.New(errMoveFailed)
			}
			return "", nil
		},
		runInDir: noopRunInDir,
	}

	wt := resolver.WorktreeInfo{Branch: branchFeature, Path: pathWtFeatureLogin}
	_, err := RenameWorktree(r, wt, "auth", wtDir, false)
	if err == nil {
		t.Fatal("expected error from move failure")
	}
}

func TestRenameWorktreeBranchRenameFails(t *testing.T) {
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			if len(args) >= 1 && args[0] == cmdRevParse {
				return "", errGitFailed
			}
			if len(args) >= 2 && args[0] == "branch" && args[1] == "-m" {
				return "", errors.New(errRenameFailed)
			}
			return "", nil
		},
		runInDir: noopRunInDir,
	}

	wt := resolver.WorktreeInfo{Branch: branchFeature, Path: pathWtFeatureLogin}
	_, err := RenameWorktree(r, wt, "auth", wtDir, false)
	if err == nil {
		t.Fatal("expected error from branch rename failure")
	}
	if !strings.Contains(err.Error(), "worktree moved but failed") {
		t.Errorf("error = %q, want 'worktree moved but failed'", err.Error())
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
	res, err := RenameWorktree(r, wt, "new-task", wtDir, false)
	if err != nil {
		t.Fatalf("RenameWorktree: %v", err)
	}
	// Should use default prefix (feature/)
	if !strings.HasPrefix(res.NewBranch, "feature/") {
		t.Errorf("NewBranch = %q, want feature/ prefix", res.NewBranch)
	}
}

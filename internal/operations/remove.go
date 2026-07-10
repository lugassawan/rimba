package operations

import (
	"context"

	"github.com/lugassawan/rimba/internal/git"
	"github.com/lugassawan/rimba/internal/progress"
	"github.com/lugassawan/rimba/internal/resolver"
)

// RemoveResult holds the outcome of a worktree removal.
type RemoveResult struct {
	Task            string
	Branch          string
	Path            string
	WorktreeRemoved bool
	BranchDeleted   bool
	BranchError     error // non-nil if worktree removed but branch delete failed
}

// RemoveWorktree removes a worktree and optionally deletes its branch.
func RemoveWorktree(ctx context.Context, r git.Runner, wt resolver.WorktreeInfo, task string, keepBranch, force bool, onProgress progress.Func) (RemoveResult, error) {
	result := RemoveResult{
		Task:   task,
		Branch: wt.Branch,
		Path:   wt.Path,
	}

	progress.Notify(onProgress, "Removing worktree...")
	if err := removeWorktreeEntry(ctx, r, wt.Path, force, wt.Prunable); err != nil {
		return result, err
	}
	result.WorktreeRemoved = true

	if !keepBranch {
		progress.Notify(onProgress, "Deleting branch...")
		if err := git.DeleteBranch(ctx, r, wt.Branch, true); err != nil {
			result.BranchError = branchDeleteFailedErr(wt.Branch, err)
			return result, nil
		}
		result.BranchDeleted = true
	}

	return result, nil
}

// removeWorktreeEntry clears the worktree's admin entry: via git worktree
// prune when prunable (its .git file was deleted out-of-band, #374, so a
// targeted git worktree remove would be refused), otherwise via a normal
// git worktree remove. Shared by RemoveWorktree and removeAndCleanup so the
// prunable/non-prunable dispatch lives in exactly one place.
func removeWorktreeEntry(ctx context.Context, r git.Runner, path string, force, prunable bool) error {
	if prunable {
		_, err := git.Prune(ctx, r, false)
		return err
	}
	return git.RemoveWorktree(ctx, r, path, force)
}

// removeAndCleanup removes a worktree and deletes its branch.
// Used by remove, merge, and clean operations. force is forwarded to git
// worktree remove so callers can discard untracked files or uncommitted
// tracked-file modifications (e.g. node_modules, local config edits).
func removeAndCleanup(ctx context.Context, r git.Runner, path, branch string, force, prunable bool) (wtRemoved, brDeleted bool, err error) {
	if err := removeWorktreeEntry(ctx, r, path, force, prunable); err != nil {
		return false, false, err
	}

	if err := git.DeleteBranch(ctx, r, branch, true); err != nil {
		return true, false, branchDeleteFailedErr(branch, err)
	}

	return true, true, nil
}

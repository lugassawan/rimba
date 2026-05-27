package operations

import (
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
func RemoveWorktree(r git.Runner, wt resolver.WorktreeInfo, task string, keepBranch, force bool, onProgress progress.Func) (RemoveResult, error) {
	result := RemoveResult{
		Task:   task,
		Branch: wt.Branch,
		Path:   wt.Path,
	}

	progress.Notify(onProgress, "Removing worktree...")
	if err := git.RemoveWorktree(r, wt.Path, force); err != nil {
		return result, err
	}
	result.WorktreeRemoved = true

	if !keepBranch {
		progress.Notify(onProgress, "Deleting branch...")
		if err := git.DeleteBranch(r, wt.Branch, true); err != nil {
			result.BranchError = branchDeleteFailedErr(wt.Branch, err)
			return result, nil
		}
		result.BranchDeleted = true
	}

	return result, nil
}

// removeAndCleanup removes a worktree and deletes its branch.
// Used by remove, merge, and clean operations.
func removeAndCleanup(r git.Runner, path, branch string) (wtRemoved, brDeleted bool, err error) {
	if err := git.RemoveWorktree(r, path, false); err != nil {
		return false, false, err
	}

	if err := git.DeleteBranch(r, branch, true); err != nil {
		return true, false, branchDeleteFailedErr(branch, err)
	}

	return true, true, nil
}

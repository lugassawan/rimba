package operations

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/lugassawan/rimba/internal/git"
	"github.com/lugassawan/rimba/internal/progress"
	"github.com/lugassawan/rimba/internal/resolver"
)

// RemoveResult holds the outcome of a worktree removal.
type RemoveResult struct {
	Task            string
	Branch          string
	Path            string
	LeftOnDisk      bool // true if the directory was left on disk (prune fallback), not fully removed
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
	defer deferSweepManifest(ctx, r, []string{wt.Path})()
	leftOnDisk, err := removeWorktreeEntry(ctx, r, wt.Path, force, wt.Prunable)
	result.LeftOnDisk = leftOnDisk
	if err != nil {
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

// removeWorktreeEntry clears the worktree's admin entry, returning whether
// the directory was left on disk. Shared by RemoveWorktree and removeAndCleanup.
func removeWorktreeEntry(ctx context.Context, r git.Runner, path string, force, prunable bool) (leftOnDisk bool, err error) {
	if prunable {
		return healAndRemoveOrphan(ctx, r, path, force)
	}

	if removeErr := git.RemoveWorktree(ctx, r, path, force); removeErr != nil {
		if !worktreeGitMissing(path) {
			return false, removeErr
		}
		return healAndRemoveOrphan(ctx, r, path, force)
	}
	return false, nil
}

// healAndRemoveOrphan repairs then retries the removal with the caller's own
// force flag, falling back to prune (dir left on disk) only if the .git
// linkfile is still missing afterward. repair's own error is ignored — git
// can exit non-zero there yet still have fixed the linkfile.
func healAndRemoveOrphan(ctx context.Context, r git.Runner, path string, force bool) (leftOnDisk bool, err error) {
	_ = git.RepairWorktree(ctx, r, path)
	removeErr := git.RemoveWorktree(ctx, r, path, force)
	if removeErr == nil {
		return false, nil
	}
	if !worktreeGitMissing(path) {
		return false, removeErr
	}
	if _, pruneErr := git.Prune(ctx, r, false); pruneErr != nil {
		return true, fmt.Errorf("remove failed after repair: %w (prune fallback also failed: %w)", removeErr, pruneErr)
	}
	return true, nil
}

// worktreeGitMissing reports whether path's .git file is absent, checked via
// a locale-independent os.Stat rather than matching git's i18n-wrapped stderr.
func worktreeGitMissing(path string) bool {
	_, err := os.Stat(filepath.Join(path, ".git"))
	return os.IsNotExist(err)
}

// removeAndCleanup removes a worktree and deletes its branch.
// Used by remove, merge, and clean operations. force is forwarded to git
// worktree remove so callers can discard untracked files or uncommitted
// tracked-file modifications (e.g. node_modules, local config edits).
func removeAndCleanup(ctx context.Context, r git.Runner, path, branch string, force, prunable bool) (wtRemoved, brDeleted, leftOnDisk bool, err error) {
	leftOnDisk, err = removeWorktreeEntry(ctx, r, path, force, prunable)
	if err != nil {
		return false, false, leftOnDisk, err
	}

	if err := git.DeleteBranch(ctx, r, branch, true); err != nil {
		return true, false, leftOnDisk, branchDeleteFailedErr(branch, err)
	}

	return true, true, leftOnDisk, nil
}

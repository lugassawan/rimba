package resolver

import (
	"path/filepath"
	"strings"
)

// BranchName returns the full branch name for a task with the given prefix.
func BranchName(prefix, task string) string {
	return prefix + task
}

// DirName converts a branch name to a directory-safe name.
// e.g. "feat/my-task" → "feat-my-task"
func DirName(branch string) string {
	return strings.ReplaceAll(branch, "/", "-")
}

// WorktreePath returns the full path to the worktree directory.
func WorktreePath(worktreeDir, branch string) string {
	return filepath.Join(worktreeDir, DirName(branch))
}

// TaskFromBranch extracts the task name from a branch name by stripping the prefix.
// Returns the full branch name if the prefix doesn't match.
func TaskFromBranch(branch, prefix string) string {
	if task, ok := strings.CutPrefix(branch, prefix); ok {
		return task
	}
	return branch
}

// WorktreeInfo holds parsed information about a worktree.
type WorktreeInfo struct {
	Path   string
	Branch string
}

// FindBranchForTask searches worktrees for one matching the given task.
// It tries: prefix+task exact match, then bare task name match.
func FindBranchForTask(task string, worktrees []WorktreeInfo, prefix string) (WorktreeInfo, bool) {
	target := BranchName(prefix, task)

	// Try exact match with prefix
	for _, wt := range worktrees {
		if wt.Branch == target {
			return wt, true
		}
	}

	// Try exact match without prefix (user passed full branch name)
	for _, wt := range worktrees {
		if wt.Branch == task {
			return wt, true
		}
	}

	return WorktreeInfo{}, false
}

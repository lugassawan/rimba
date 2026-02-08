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
// e.g. "feature/my-task" → "feature-my-task"
func DirName(branch string) string {
	return strings.ReplaceAll(branch, "/", "-")
}

// WorktreePath returns the full path to the worktree directory.
func WorktreePath(worktreeDir, branch string) string {
	return filepath.Join(worktreeDir, DirName(branch))
}

// TaskFromBranch extracts the task name from a branch by trying each prefix in order.
// Returns the task name and the matched prefix string.
// If no prefix matches, returns the full branch name and an empty string.
func TaskFromBranch(branch string, prefixes []string) (task, matchedPrefix string) {
	for _, p := range prefixes {
		if t, ok := strings.CutPrefix(branch, p); ok {
			return t, p
		}
	}
	return branch, ""
}

// WorktreeInfo holds parsed information about a worktree.
type WorktreeInfo struct {
	Path   string
	Branch string
}

// FindBranchForTask searches worktrees for one matching the given task.
// It tries each prefix+task combination, then falls back to exact branch name match.
func FindBranchForTask(task string, worktrees []WorktreeInfo, prefixes []string) (WorktreeInfo, bool) {
	for _, p := range prefixes {
		target := BranchName(p, task)
		for _, wt := range worktrees {
			if wt.Branch == target {
				return wt, true
			}
		}
	}

	for _, wt := range worktrees {
		if wt.Branch == task {
			return wt, true
		}
	}

	return WorktreeInfo{}, false
}

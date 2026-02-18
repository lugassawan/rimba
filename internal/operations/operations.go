package operations

import (
	"fmt"

	"github.com/lugassawan/rimba/internal/git"
	"github.com/lugassawan/rimba/internal/resolver"
)

const errWorktreeNotFound = "worktree not found for task %q"

// ListWorktreeInfos converts git worktree entries to resolver-compatible WorktreeInfo slice.
func ListWorktreeInfos(r git.Runner) ([]resolver.WorktreeInfo, error) {
	entries, err := git.ListWorktrees(r)
	if err != nil {
		return nil, err
	}

	worktrees := make([]resolver.WorktreeInfo, len(entries))
	for i, e := range entries {
		worktrees[i] = resolver.WorktreeInfo{
			Path:   e.Path,
			Branch: e.Branch,
		}
	}
	return worktrees, nil
}

// FindWorktree looks up a worktree by task name.
func FindWorktree(r git.Runner, task string) (resolver.WorktreeInfo, error) {
	worktrees, err := ListWorktreeInfos(r)
	if err != nil {
		return resolver.WorktreeInfo{}, err
	}

	wt, found := resolver.FindBranchForTask(task, worktrees, resolver.AllPrefixes())
	if !found {
		return resolver.WorktreeInfo{}, fmt.Errorf(errWorktreeNotFound, task)
	}
	return wt, nil
}

// FilterByType returns worktrees whose branch prefix matches the given type string.
// For example, typeStr "feature" matches branches with prefix "feature/".
func FilterByType(worktrees []resolver.WorktreeInfo, prefixes []string, typeStr string) []resolver.WorktreeInfo {
	target := typeStr + "/"
	var out []resolver.WorktreeInfo
	for _, wt := range worktrees {
		_, matchedPrefix := resolver.TaskFromBranch(wt.Branch, prefixes)
		if matchedPrefix == target {
			out = append(out, wt)
		}
	}
	return out
}

package operations

import (
	"fmt"

	"github.com/lugassawan/rimba/internal/git"
	"github.com/lugassawan/rimba/internal/resolver"
)

// ErrWorktreeNotFoundFmt is a format string for worktree-not-found errors.
const ErrWorktreeNotFoundFmt = "worktree not found for task %q"

// ResolveMainBranch returns the main branch name.
// If configDefault is non-empty it is used directly; otherwise git detection is used.
func ResolveMainBranch(r git.Runner, configDefault string) (string, error) {
	if configDefault != "" {
		return configDefault, nil
	}
	return git.DefaultBranch(r)
}

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
		return resolver.WorktreeInfo{}, fmt.Errorf(ErrWorktreeNotFoundFmt, task)
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

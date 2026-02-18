package operations

import (
	"github.com/lugassawan/rimba/internal/git"
	"github.com/lugassawan/rimba/internal/resolver"
)

// SyncBranch synchronises a worktree with the main branch using rebase or merge.
// On rebase failure the failed rebase is aborted so the worktree stays clean.
func SyncBranch(r git.Runner, dir, mainBranch string, useMerge bool) error {
	if useMerge {
		return git.Merge(r, dir, mainBranch, false)
	}
	if err := git.Rebase(r, dir, mainBranch); err != nil {
		_ = git.AbortRebase(r, dir)
		return err
	}
	return nil
}

// CollectTasks extracts the task name from each worktree branch using the given prefixes.
func CollectTasks(worktrees []resolver.WorktreeInfo, prefixes []string) []string {
	tasks := make([]string, 0, len(worktrees))
	for _, wt := range worktrees {
		task, _ := resolver.TaskFromBranch(wt.Branch, prefixes)
		tasks = append(tasks, task)
	}
	return tasks
}

// FilterEligible returns worktrees eligible for sync (excludes main branch, detached,
// and optionally inherited worktrees).
func FilterEligible(worktrees []resolver.WorktreeInfo, prefixes []string, mainBranch string, allTasks []string, includeInherited bool) []resolver.WorktreeInfo {
	var eligible []resolver.WorktreeInfo
	for _, wt := range worktrees {
		if wt.Branch == mainBranch || wt.Branch == "" {
			continue
		}
		task, _ := resolver.TaskFromBranch(wt.Branch, prefixes)
		if !includeInherited && resolver.IsInherited(task, allTasks) {
			continue
		}
		eligible = append(eligible, wt)
	}
	return eligible
}

// SyncMethodLabel returns a past-tense label for the sync method used.
func SyncMethodLabel(useMerge bool) string {
	if useMerge {
		return "Merged"
	}
	return "Rebased"
}

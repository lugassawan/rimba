package operations

import (
	"context"

	"github.com/lugassawan/rimba/internal/git"
	"github.com/lugassawan/rimba/internal/resolver"
)

// CollectWorktreeStatus gathers dirty/ahead/behind state for a worktree path.
// Returns Unknown=true on any git failure (including timeout) rather than silently clean.
func CollectWorktreeStatus(ctx context.Context, r git.Runner, wtPath string) resolver.WorktreeStatus {
	dirty, dirtyErr := git.IsDirty(ctx, r, wtPath)
	ahead, behind, aheadBehindErr := git.AheadBehind(ctx, r, wtPath)

	if dirtyErr != nil || aheadBehindErr != nil {
		return resolver.WorktreeStatus{Unknown: true}
	}

	return resolver.WorktreeStatus{
		Dirty:  dirty,
		Ahead:  ahead,
		Behind: behind,
	}
}

// FilterDetailsByStatus filters worktree details by dirty and/or behind status.
// When filterDirty is true, only dirty worktrees are kept.
// When filterBehind is true, only worktrees behind upstream are kept.
// Worktrees with Unknown=true are always kept regardless of filter flags.
func FilterDetailsByStatus(rows []resolver.WorktreeDetail, filterDirty, filterBehind bool) []resolver.WorktreeDetail {
	filtered := rows[:0]
	for _, row := range rows {
		if row.Status.Unknown {
			filtered = append(filtered, row)
			continue
		}
		if filterDirty && !row.Status.Dirty {
			continue
		}
		if filterBehind && row.Status.Behind == 0 {
			continue
		}
		filtered = append(filtered, row)
	}
	return filtered
}

package operations

import (
	"github.com/lugassawan/rimba/internal/git"
	"github.com/lugassawan/rimba/internal/resolver"
)

// CollectWorktreeStatus gathers dirty/ahead/behind state for a worktree path.
func CollectWorktreeStatus(r git.Runner, wtPath string) resolver.WorktreeStatus {
	var status resolver.WorktreeStatus

	if dirty, err := git.IsDirty(r, wtPath); err == nil && dirty {
		status.Dirty = true
	}

	ahead, behind, _ := git.AheadBehind(r, wtPath)
	status.Ahead = ahead
	status.Behind = behind

	return status
}

// FilterDetailsByStatus filters worktree details by dirty and/or behind status.
// When filterDirty is true, only dirty worktrees are kept.
// When filterBehind is true, only worktrees behind upstream are kept.
func FilterDetailsByStatus(rows []resolver.WorktreeDetail, filterDirty, filterBehind bool) []resolver.WorktreeDetail {
	filtered := rows[:0]
	for _, row := range rows {
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

package operations

import (
	"context"
	"sort"
	"sync"
	"time"

	"github.com/lugassawan/rimba/internal/fsutil"
	"github.com/lugassawan/rimba/internal/git"
	"github.com/lugassawan/rimba/internal/parallel"
	"github.com/lugassawan/rimba/internal/resolver"
)

const recentWindow = 7 * 24 * time.Hour

// withItemTimeout and dirSizeFn are indirections over git.WithItemTimeout and
// fsutil.DirSize so tests can substitute a shrunk timeout and a slow walk.
// Mutating these in a test is not safe under t.Parallel().
var (
	withItemTimeout = git.WithItemTimeout
	dirSizeFn       = fsutil.DirSize
)

// StatusEntry holds per-worktree data collected during a status dashboard run.
type StatusEntry struct {
	Entry      git.WorktreeEntry
	Status     resolver.WorktreeStatus
	CommitTime time.Time
	HasTime    bool
	SizeBytes  *int64
	Recent7D   *int
}

// StatusSummary is the aggregate counts across all dashboard entries.
type StatusSummary struct {
	Total, Dirty, Stale, Behind int
}

// StatusDashboardRequest configures a StatusDashboard call.
type StatusDashboardRequest struct {
	Detail bool // when true, fan-out DirSize + 7D-velocity, build Footprint, sort by size desc
}

// StatusDashboardResult carries entries and optional disk footprint.
type StatusDashboardResult struct {
	Entries   []StatusEntry  // empty when no worktree candidates
	Footprint *DiskFootprint // nil unless req.Detail
}

// StatusDashboard is the shared pipeline for the `rimba status` dashboard:
// resolve main → list → filter → parallel collect → optional disk-size
// fan-out → BuildDiskFootprint → sort by size desc.
//
// Spinner is a UI concern excluded from this layer; callers wrap the call
// with spinner.Start/Stop as needed.
func StatusDashboard(ctx context.Context, gitR git.Runner, req StatusDashboardRequest) (StatusDashboardResult, error) {
	if err := ctx.Err(); err != nil {
		return StatusDashboardResult{}, err
	}
	mainBranch, err := ResolveMainBranch(ctx, gitR, "")
	if err != nil {
		return StatusDashboardResult{}, err
	}

	allEntries, err := git.ListWorktrees(ctx, gitR)
	if err != nil {
		return StatusDashboardResult{}, err
	}

	mainEntry := git.FindEntry(allEntries, mainBranch)
	candidates := git.FilterEntries(allEntries, mainBranch)

	if len(candidates) == 0 {
		return StatusDashboardResult{}, nil
	}

	var mainSize int64
	var mainErr error
	var mainWG sync.WaitGroup
	if req.Detail && mainEntry != nil {
		mainWG.Add(1)
		go func(path string) {
			defer mainWG.Done()
			mainSize, mainErr = fsutil.DirSize(ctx, path)
		}(mainEntry.Path)
	}

	entries := collectStatusEntries(ctx, gitR, candidates, req.Detail)
	mainWG.Wait()

	var footprint *DiskFootprint
	if req.Detail {
		sizes := make([]*int64, len(entries))
		for i, e := range entries {
			sizes[i] = e.SizeBytes
		}
		fp := BuildDiskFootprint(sizes, mainSize, mainErr)
		footprint = &fp
		sortEntriesBySizeDesc(entries)
	}

	return StatusDashboardResult{Entries: entries, Footprint: footprint}, nil
}

// SummarizeStatus counts dirty, stale, and behind entries.
// An entry is stale only when HasTime is true and CommitTime is before staleThreshold.
func SummarizeStatus(entries []StatusEntry, staleThreshold time.Time) StatusSummary {
	s := StatusSummary{Total: len(entries)}
	for _, e := range entries {
		if e.Status.Dirty {
			s.Dirty++
		}
		if e.Status.Behind > 0 {
			s.Behind++
		}
		if e.HasTime && e.CommitTime.Before(staleThreshold) {
			s.Stale++
		}
	}
	return s
}

// collectStatusEntries gathers dirty/ahead/behind state and last commit time
// per candidate in parallel. Under detail it also computes size and 7-day
// velocity. Each per-item operation gets its own withItemTimeout budget, so a
// slow one (e.g. a large DirSize walk) cannot starve the others; per-item
// errors leave the pointer nil (non-fatal). This trades a lower worst-case
// per-candidate latency (one shared budget) for correctness (independent
// budgets): a candidate where every op stalls can now take up to 4x
// itemQueryTimeout instead of 1x.
func collectStatusEntries(ctx context.Context, gitR git.Runner, candidates []git.WorktreeEntry, detail bool) []StatusEntry {
	return parallel.Collect(ctx, len(candidates), 8, func(ctx context.Context, i int) StatusEntry {
		e := candidates[i]

		statusCtx, cancelStatus := withItemTimeout(ctx)
		st := CollectWorktreeStatus(statusCtx, gitR, e.Path)
		cancelStatus()

		timeCtx, cancelTime := withItemTimeout(ctx)
		var ct time.Time
		var hasTime bool
		if t, err := git.LastCommitTime(timeCtx, gitR, e.Branch); err == nil {
			ct = t
			hasTime = true
		}
		cancelTime()

		se := StatusEntry{Entry: e, Status: st, CommitTime: ct, HasTime: hasTime}
		if detail {
			sizeCtx, cancelSize := withItemTimeout(ctx)
			if n, err := dirSizeFn(sizeCtx, e.Path); err == nil {
				se.SizeBytes = &n
			}
			cancelSize()

			countCtx, cancelCount := withItemTimeout(ctx)
			if c, err := git.CommitCountSince(countCtx, gitR, e.Branch, recentWindow); err == nil {
				se.Recent7D = &c
			}
			cancelCount()
		}
		return se
	})
}

// sortEntriesBySizeDesc sorts largest first, stable, nils last.
func sortEntriesBySizeDesc(entries []StatusEntry) {
	sort.SliceStable(entries, func(i, j int) bool {
		a, b := entries[i].SizeBytes, entries[j].SizeBytes
		switch {
		case a == nil && b == nil:
			return false
		case a == nil:
			return false
		case b == nil:
			return true
		default:
			return *a > *b
		}
	})
}

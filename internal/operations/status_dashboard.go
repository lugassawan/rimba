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
func StatusDashboard(_ context.Context, gitR git.Runner, req StatusDashboardRequest) (StatusDashboardResult, error) {
	mainBranch, err := ResolveMainBranch(gitR, "")
	if err != nil {
		return StatusDashboardResult{}, err
	}

	allEntries, err := git.ListWorktrees(gitR)
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
			mainSize, mainErr = fsutil.DirSize(path)
		}(mainEntry.Path)
	}

	entries := collectStatusEntries(gitR, candidates, req.Detail)
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
// velocity; per-item errors leave the pointer nil (non-fatal).
func collectStatusEntries(gitR git.Runner, candidates []git.WorktreeEntry, detail bool) []StatusEntry {
	return parallel.Collect(len(candidates), 8, func(i int) StatusEntry {
		e := candidates[i]
		st := CollectWorktreeStatus(gitR, e.Path)
		var ct time.Time
		var hasTime bool
		if t, err := git.LastCommitTime(gitR, e.Branch); err == nil {
			ct = t
			hasTime = true
		}
		se := StatusEntry{Entry: e, Status: st, CommitTime: ct, HasTime: hasTime}
		if detail {
			if n, err := fsutil.DirSize(e.Path); err == nil {
				se.SizeBytes = &n
			}
			if c, err := git.CommitCountSince(gitR, e.Branch, recentWindow); err == nil {
				se.Recent7D = &c
			}
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

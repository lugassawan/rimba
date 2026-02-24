package operations

import (
	"fmt"
	"time"

	"github.com/lugassawan/rimba/internal/git"
)

// CleanCandidate holds a branch/path pair eligible for removal.
type CleanCandidate struct {
	Path   string
	Branch string
}

// StaleCandidate extends CleanCandidate with the last commit time.
type StaleCandidate struct {
	CleanCandidate
	LastCommit time.Time
}

// CleanedItem holds the outcome of removing a single worktree.
type CleanedItem struct {
	Branch          string
	Path            string
	WorktreeRemoved bool
	BranchDeleted   bool
	Error           error // non-nil if removal or branch deletion failed
}

// MergedResult holds the candidates and any warnings from merged-branch detection.
type MergedResult struct {
	Candidates []CleanCandidate
	Warnings   []string
}

// StaleResult holds the candidates and any warnings from stale-branch detection.
type StaleResult struct {
	Candidates []StaleCandidate
	Warnings   []string
}

// FindMergedCandidates returns worktrees whose branches are merged into mergeRef.
// It checks both regular merges and squash-merges.
func FindMergedCandidates(r git.Runner, mergeRef, mainBranch string) (MergedResult, error) {
	mergedList, err := git.MergedBranches(r, mergeRef)
	if err != nil {
		return MergedResult{}, fmt.Errorf("failed to list merged branches: %w", err)
	}

	mergedSet := make(map[string]bool, len(mergedList))
	for _, b := range mergedList {
		mergedSet[b] = true
	}

	entries, err := git.ListWorktrees(r)
	if err != nil {
		return MergedResult{}, err
	}

	var result MergedResult
	for _, e := range git.FilterEntries(entries, mainBranch) {
		if mergedSet[e.Branch] {
			result.Candidates = append(result.Candidates, CleanCandidate{Path: e.Path, Branch: e.Branch})
			continue
		}

		// Fallback: squash-merge detection
		squashed, err := git.IsSquashMerged(r, mergeRef, e.Branch)
		if err != nil {
			result.Warnings = append(result.Warnings, fmt.Sprintf("skipped %s: squash-merge check failed: %v", e.Branch, err))
			continue
		}
		if squashed {
			result.Candidates = append(result.Candidates, CleanCandidate{Path: e.Path, Branch: e.Branch})
		}
	}
	return result, nil
}

// FindStaleCandidates returns worktrees with no commits in the last staleDays days.
func FindStaleCandidates(r git.Runner, mainBranch string, staleDays int) (StaleResult, error) {
	entries, err := git.ListWorktrees(r)
	if err != nil {
		return StaleResult{}, err
	}

	threshold := time.Now().Add(-time.Duration(staleDays) * 24 * time.Hour)

	var result StaleResult
	for _, e := range git.FilterEntries(entries, mainBranch) {
		ct, err := git.LastCommitTime(r, e.Branch)
		if err != nil {
			result.Warnings = append(result.Warnings, fmt.Sprintf("skipped %s: %v", e.Branch, err))
			continue
		}

		if ct.Before(threshold) {
			result.Candidates = append(result.Candidates, StaleCandidate{
				CleanCandidate: CleanCandidate{Path: e.Path, Branch: e.Branch},
				LastCommit:     ct,
			})
		}
	}
	return result, nil
}

// RemoveCandidates removes worktrees and their branches, returning the outcome of each.
func RemoveCandidates(r git.Runner, candidates []CleanCandidate, onProgress ProgressFunc) []CleanedItem {
	items := make([]CleanedItem, 0, len(candidates))
	for _, c := range candidates {
		notify(onProgress, fmt.Sprintf("Removing %s...", c.Branch))
		wtRemoved, brDeleted, err := removeAndCleanup(r, c.Path, c.Branch)
		items = append(items, CleanedItem{
			Branch:          c.Branch,
			Path:            c.Path,
			WorktreeRemoved: wtRemoved,
			BranchDeleted:   brDeleted,
			Error:           err,
		})
	}
	return items
}

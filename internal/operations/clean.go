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
}

// FindMergedCandidates returns worktrees whose branches are merged into mergeRef.
// It checks both regular merges and squash-merges.
func FindMergedCandidates(r git.Runner, mergeRef, mainBranch string) ([]CleanCandidate, error) {
	mergedList, err := git.MergedBranches(r, mergeRef)
	if err != nil {
		return nil, fmt.Errorf("failed to list merged branches: %w", err)
	}

	mergedSet := make(map[string]bool, len(mergedList))
	for _, b := range mergedList {
		mergedSet[b] = true
	}

	entries, err := git.ListWorktrees(r)
	if err != nil {
		return nil, err
	}

	var candidates []CleanCandidate
	for _, e := range git.FilterEntries(entries, mainBranch) {
		if mergedSet[e.Branch] {
			candidates = append(candidates, CleanCandidate{Path: e.Path, Branch: e.Branch})
			continue
		}

		// Fallback: squash-merge detection
		squashed, err := git.IsSquashMerged(r, mergeRef, e.Branch)
		if err != nil {
			continue
		}
		if squashed {
			candidates = append(candidates, CleanCandidate{Path: e.Path, Branch: e.Branch})
		}
	}
	return candidates, nil
}

// FindStaleCandidates returns worktrees with no commits in the last staleDays days.
func FindStaleCandidates(r git.Runner, mainBranch string, staleDays int) ([]StaleCandidate, error) {
	entries, err := git.ListWorktrees(r)
	if err != nil {
		return nil, err
	}

	threshold := time.Now().Add(-time.Duration(staleDays) * 24 * time.Hour)

	var candidates []StaleCandidate
	for _, e := range git.FilterEntries(entries, mainBranch) {
		ct, err := git.LastCommitTime(r, e.Branch)
		if err != nil {
			continue
		}

		if ct.Before(threshold) {
			candidates = append(candidates, StaleCandidate{
				CleanCandidate: CleanCandidate{Path: e.Path, Branch: e.Branch},
				LastCommit:     ct,
			})
		}
	}
	return candidates, nil
}

// RemoveCandidates removes worktrees and their branches, returning the outcome of each.
func RemoveCandidates(r git.Runner, candidates []CleanCandidate, onProgress ProgressFunc) []CleanedItem {
	items := make([]CleanedItem, 0, len(candidates))
	for _, c := range candidates {
		notify(onProgress, fmt.Sprintf("Removing %s...", c.Branch))
		wtRemoved, brDeleted, _ := removeAndCleanup(r, c.Path, c.Branch)
		items = append(items, CleanedItem{
			Branch:          c.Branch,
			Path:            c.Path,
			WorktreeRemoved: wtRemoved,
			BranchDeleted:   brDeleted,
		})
	}
	return items
}

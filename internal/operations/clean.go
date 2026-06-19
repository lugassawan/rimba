package operations

import (
	"context"
	"fmt"
	"time"

	"github.com/lugassawan/rimba/internal/errhint"
	"github.com/lugassawan/rimba/internal/git"
	"github.com/lugassawan/rimba/internal/progress"
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
	RemoteDeleted   bool  // true if the remote branch was successfully deleted
	RemoteError     error // non-nil if remote branch deletion failed
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
func FindMergedCandidates(ctx context.Context, r git.Runner, mergeRef, mainBranch string) (MergedResult, error) {
	mergedList, err := git.MergedBranches(ctx, r, mergeRef)
	if err != nil {
		return MergedResult{}, errhint.WithFix(
			fmt.Errorf("failed to list merged branches: %w", err),
			"check that the main branch exists: git branch --list main",
		)
	}

	mergedSet := make(map[string]bool, len(mergedList))
	for _, b := range mergedList {
		mergedSet[b] = true
	}

	entries, err := git.ListWorktrees(ctx, r)
	if err != nil {
		return MergedResult{}, err
	}

	var result MergedResult
	for _, e := range git.FilterEntries(entries, mainBranch) {
		if mergedSet[e.Branch] {
			// `git branch --merged` lists every branch reachable from mergeRef,
			// including a fresh worktree branch whose tip *is* the base commit.
			// Guard against removing such branches: only treat a --merged hit as
			// a candidate when the branch actually contributed commits of its own.
			hasOwn, err := git.HasOwnCommits(ctx, r, mergeRef, e.Branch)
			if err != nil {
				result.Warnings = append(result.Warnings, fmt.Sprintf("skipped %s: own-commits check failed: %v", e.Branch, err))
				continue
			}
			if hasOwn {
				result.Candidates = append(result.Candidates, CleanCandidate{Path: e.Path, Branch: e.Branch})
			}
			continue
		}

		// Fallback: squash-merge detection
		squashed, err := git.IsSquashMerged(ctx, r, mergeRef, e.Branch)
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
func FindStaleCandidates(ctx context.Context, r git.Runner, mainBranch string, staleDays int) (StaleResult, error) {
	entries, err := git.ListWorktrees(ctx, r)
	if err != nil {
		return StaleResult{}, err
	}

	threshold := time.Now().Add(-time.Duration(staleDays) * 24 * time.Hour)

	var result StaleResult
	for _, e := range git.FilterEntries(entries, mainBranch) {
		ct, err := git.LastCommitTime(ctx, r, e.Branch)
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
// When originPresent is true the remote branch is also deleted after the worktree is
// removed. Callers are responsible for probing RemoteExists before invoking — passing
// a pre-resolved boolean avoids redundant git remote get-url calls per candidate and
// ensures the CLI dry-run preview and actual deletion share a single probe result.
// force is forwarded to git worktree remove to allow discarding untracked files.
func RemoveCandidates(ctx context.Context, r git.Runner, candidates []CleanCandidate, originPresent bool, force bool, onProgress progress.Func) []CleanedItem {
	items := make([]CleanedItem, 0, len(candidates))
	for _, c := range candidates {
		progress.Notifyf(onProgress, "Removing %s...", c.Branch)
		wtRemoved, brDeleted, err := removeAndCleanup(ctx, r, c.Path, c.Branch, force)
		item := CleanedItem{
			Branch:          c.Branch,
			Path:            c.Path,
			WorktreeRemoved: wtRemoved,
			BranchDeleted:   brDeleted,
			Error:           err,
		}
		if originPresent && wtRemoved {
			deleteRemoteForItem(ctx, r, c.Branch, &item)
		}
		items = append(items, item)
	}
	return items
}

// deleteRemoteForItem deletes the remote branch on git.DefaultRemote.
// Caller must verify the remote exists before invoking.
func deleteRemoteForItem(ctx context.Context, r git.Runner, branch string, item *CleanedItem) {
	if err := git.DeleteRemoteBranch(ctx, r, git.DefaultRemote, branch); err != nil {
		item.RemoteError = err
		return
	}
	item.RemoteDeleted = true
}

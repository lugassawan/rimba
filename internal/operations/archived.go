package operations

import (
	"fmt"

	"github.com/lugassawan/rimba/internal/git"
	"github.com/lugassawan/rimba/internal/resolver"
)

// FindArchivedBranch finds a branch for the given task that is not associated
// with any active worktree. It tries prefix+task combinations first, then exact
// match, then falls back to task extraction.
func FindArchivedBranch(r git.Runner, task string) (string, error) {
	branches, err := git.LocalBranches(r)
	if err != nil {
		return "", fmt.Errorf("list branches: %w", err)
	}

	active, err := buildActiveSet(r)
	if err != nil {
		return "", err
	}

	prefixes := resolver.AllPrefixes()

	// Try prefix+task combinations first
	for _, p := range prefixes {
		candidate := resolver.BranchName(p, task)
		for _, b := range branches {
			if b == candidate && !active[b] {
				return b, nil
			}
		}
	}

	// Fallback: exact match
	for _, b := range branches {
		if b == task && !active[b] {
			return b, nil
		}
	}

	// Fallback: match by task extraction
	for _, b := range branches {
		if active[b] {
			continue
		}
		t, _ := resolver.TaskFromBranch(b, prefixes)
		if t == task {
			return b, nil
		}
	}

	return "", fmt.Errorf("no archived branch found for task %q\nTo see archived branches: rimba list --archived", task)
}

// ListArchivedBranches returns branches that are not associated with any active
// worktree and not the main branch.
func ListArchivedBranches(r git.Runner, mainBranch string) ([]string, error) {
	branches, err := git.LocalBranches(r)
	if err != nil {
		return nil, err
	}

	active, err := buildActiveSet(r)
	if err != nil {
		return nil, err
	}

	var archived []string
	for _, b := range branches {
		if b == mainBranch || active[b] {
			continue
		}
		archived = append(archived, b)
	}
	return archived, nil
}

// buildActiveSet returns a set of branch names that are checked out in active worktrees.
func buildActiveSet(r git.Runner) (map[string]bool, error) {
	entries, err := git.ListWorktrees(r)
	if err != nil {
		return nil, fmt.Errorf("list worktrees: %w", err)
	}

	active := make(map[string]bool, len(entries))
	for _, e := range entries {
		if e.Branch != "" {
			active[e.Branch] = true
		}
	}
	return active, nil
}

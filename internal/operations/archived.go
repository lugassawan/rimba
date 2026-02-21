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

	if b, ok := searchByPrefixedTask(branches, active, prefixes, task); ok {
		return b, nil
	}
	if b, ok := searchByExactMatch(branches, active, task); ok {
		return b, nil
	}
	if b, ok := searchByTaskExtraction(branches, active, prefixes, task); ok {
		return b, nil
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

// searchByPrefixedTask tries prefix+task combinations against branches.
func searchByPrefixedTask(branches []string, active map[string]bool, prefixes []string, task string) (string, bool) {
	for _, p := range prefixes {
		candidate := resolver.BranchName(p, task)
		for _, b := range branches {
			if b == candidate && !active[b] {
				return b, true
			}
		}
	}
	return "", false
}

// searchByExactMatch checks if the task name exactly matches an inactive branch.
func searchByExactMatch(branches []string, active map[string]bool, task string) (string, bool) {
	for _, b := range branches {
		if b == task && !active[b] {
			return b, true
		}
	}
	return "", false
}

// searchByTaskExtraction finds a branch whose extracted task matches the given task.
func searchByTaskExtraction(branches []string, active map[string]bool, prefixes []string, task string) (string, bool) {
	for _, b := range branches {
		if active[b] {
			continue
		}
		t, _ := resolver.TaskFromBranch(b, prefixes)
		if t == task {
			return b, true
		}
	}
	return "", false
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

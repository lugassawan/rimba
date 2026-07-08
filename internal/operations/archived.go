package operations

import (
	"context"
	"fmt"

	"github.com/lugassawan/rimba/internal/config"
	"github.com/lugassawan/rimba/internal/git"
	"github.com/lugassawan/rimba/internal/resolver"
)

// FindArchivedBranch finds a branch for the given service+task that is not
// associated with any active worktree. service may be empty for non-monorepo repos.
func FindArchivedBranch(ctx context.Context, r git.Runner, service, task string) (string, error) {
	branches, err := git.LocalBranches(ctx, r)
	if err != nil {
		return "", fmt.Errorf("list branches: %w", err)
	}

	active, err := buildActiveSet(ctx, r)
	if err != nil {
		return "", err
	}

	prefixes := config.PrefixSetFromContext(ctx).Strip()

	if b, ok := searchByPrefixedTask(branches, active, prefixes, service, task); ok {
		return b, nil
	}
	if b, ok := searchByExactMatch(branches, active, service, task); ok {
		return b, nil
	}
	if b, ok := searchByTaskExtraction(branches, active, prefixes, service, task); ok {
		return b, nil
	}

	return "", fmt.Errorf("no archived branch found for task %q\nTo see archived branches: rimba list --archived", task)
}

// ListArchivedBranches returns branches that are not associated with any active
// worktree and not the main branch.
func ListArchivedBranches(ctx context.Context, r git.Runner, mainBranch string) ([]string, error) {
	branches, err := git.LocalBranches(ctx, r)
	if err != nil {
		return nil, err
	}

	active, err := buildActiveSet(ctx, r)
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
// For monorepo (service non-empty), candidates include the service segment.
func searchByPrefixedTask(branches []string, active map[string]bool, prefixes []string, service, task string) (string, bool) {
	for _, p := range prefixes {
		candidate := resolver.FullBranchName(service, p, task)
		for _, b := range branches {
			if b == candidate && !active[b] {
				return b, true
			}
		}
	}
	return "", false
}

// searchByExactMatch checks if the task name exactly matches an inactive branch.
// For monorepo (service non-empty), also tries service+"/"+task as an exact branch name.
func searchByExactMatch(branches []string, active map[string]bool, service, task string) (string, bool) {
	for _, b := range branches {
		if !active[b] && (b == task || (service != "" && b == service+"/"+task)) {
			return b, true
		}
	}
	return "", false
}

// searchByTaskExtraction finds a branch whose extracted task matches the given task.
// For monorepo (service non-empty), also verifies the branch's service segment matches.
func searchByTaskExtraction(branches []string, active map[string]bool, prefixes []string, service, task string) (string, bool) {
	for _, b := range branches {
		if active[b] {
			continue
		}
		t, _ := resolver.PureTaskFromBranch(b, prefixes)
		if t != task {
			continue
		}
		if service != "" {
			svc, _, _ := resolver.ServiceFromBranch(b, prefixes)
			if svc != service {
				continue
			}
		}
		return b, true
	}
	return "", false
}

// buildActiveSet returns a set of branch names that are checked out in active worktrees.
func buildActiveSet(ctx context.Context, r git.Runner) (map[string]bool, error) {
	entries, err := git.ListWorktrees(ctx, r)
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

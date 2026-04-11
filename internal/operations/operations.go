package operations

import (
	"fmt"
	"strings"

	"github.com/lugassawan/rimba/internal/git"
	"github.com/lugassawan/rimba/internal/resolver"
)

// ErrWorktreeNotFoundFmt is a format string for worktree-not-found errors.
const ErrWorktreeNotFoundFmt = "worktree not found for task %q"

// ResolveMainBranch returns the main branch name.
// If configDefault is non-empty it is used directly; otherwise git detection is used.
func ResolveMainBranch(r git.Runner, configDefault string) (string, error) {
	if configDefault != "" {
		return configDefault, nil
	}
	return git.DefaultBranch(r)
}

// ListWorktreeInfos converts git worktree entries to resolver-compatible WorktreeInfo slice.
func ListWorktreeInfos(r git.Runner) ([]resolver.WorktreeInfo, error) {
	entries, err := git.ListWorktrees(r)
	if err != nil {
		return nil, err
	}

	prefixes := resolver.AllPrefixes()
	worktrees := make([]resolver.WorktreeInfo, len(entries))
	for i, e := range entries {
		svc, _, _ := resolver.ServiceFromBranch(e.Branch, prefixes)
		worktrees[i] = resolver.WorktreeInfo{
			Path:    e.Path,
			Branch:  e.Branch,
			Service: svc,
		}
	}
	return worktrees, nil
}

// FindWorktree looks up a worktree by service and task name.
// When service is empty and the task matches multiple services, an ambiguity error is returned.
func FindWorktree(r git.Runner, service, task string) (resolver.WorktreeInfo, error) {
	worktrees, err := ListWorktreeInfos(r)
	if err != nil {
		return resolver.WorktreeInfo{}, err
	}

	prefixes := resolver.AllPrefixes()
	wt, found := resolver.FindBranchForTask(service, task, worktrees, prefixes)
	if !found {
		if service == "" {
			matches := resolver.FindAllBranchesForTask(task, worktrees, prefixes)
			if len(matches) > 1 {
				return resolver.WorktreeInfo{}, ambiguityError(task, matches)
			}
		}
		return resolver.WorktreeInfo{}, fmt.Errorf(ErrWorktreeNotFoundFmt, task)
	}
	return wt, nil
}

// FilterByType returns worktrees whose branch prefix matches the given type string.
// For example, typeStr "feature" matches branches with prefix "feature/".
func FilterByType(worktrees []resolver.WorktreeInfo, prefixes []string, typeStr string) []resolver.WorktreeInfo {
	target := typeStr + "/"
	var out []resolver.WorktreeInfo
	for _, wt := range worktrees {
		_, matchedPrefix := resolver.TaskFromBranch(wt.Branch, prefixes)
		if matchedPrefix == target {
			out = append(out, wt)
		}
	}
	return out
}

// ambiguityError builds a descriptive error when a bare task matches multiple services.
func ambiguityError(task string, matches []resolver.WorktreeInfo) error {
	branches := make([]string, len(matches))
	for i, m := range matches {
		branches[i] = "  " + m.Branch
	}
	return fmt.Errorf("multiple worktrees match %q:\n%s\nSpecify the service: rimba <command> <service>/%s",
		task, strings.Join(branches, "\n"), task)
}

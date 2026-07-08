package operations

import (
	"context"
	"fmt"
	"strings"

	"github.com/lugassawan/rimba/internal/config"
	"github.com/lugassawan/rimba/internal/errhint"
	"github.com/lugassawan/rimba/internal/git"
	"github.com/lugassawan/rimba/internal/resolver"
)

// ErrWorktreeNotFoundFmt is a format string for worktree-not-found errors.
const ErrWorktreeNotFoundFmt = "worktree not found for task %q"

// ResolveMainBranch returns the main branch name.
// If configDefault is non-empty it is used directly; otherwise git detection is used.
func ResolveMainBranch(ctx context.Context, r git.Runner, configDefault string) (string, error) {
	if configDefault != "" {
		return configDefault, nil
	}
	return git.DefaultBranch(ctx, r)
}

// ListWorktreeInfos converts git worktree entries to resolver-compatible WorktreeInfo slice.
func ListWorktreeInfos(ctx context.Context, r git.Runner) ([]resolver.WorktreeInfo, error) {
	entries, err := git.ListWorktrees(ctx, r)
	if err != nil {
		return nil, err
	}

	prefixes := config.PrefixSetFromContext(ctx).Strip()
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
func FindWorktree(ctx context.Context, r git.Runner, service, task string) (resolver.WorktreeInfo, error) {
	worktrees, err := ListWorktreeInfos(ctx, r)
	if err != nil {
		return resolver.WorktreeInfo{}, err
	}

	prefixes := config.PrefixSetFromContext(ctx).Strip()
	wt, found := resolver.FindBranchForTask(service, task, worktrees, prefixes)
	if !found {
		if service == "" {
			matches := resolver.FindAllBranchesForTask(task, worktrees, prefixes)
			if len(matches) > 1 {
				return resolver.WorktreeInfo{}, ambiguityError(task, matches)
			}
		}
		return resolver.WorktreeInfo{}, errhint.WithFix(
			fmt.Errorf(ErrWorktreeNotFoundFmt, task),
			"run: rimba list  to see available worktrees",
		)
	}
	return wt, nil
}

// FilterByType returns worktrees whose branch prefix matches the given type string.
// For example, typeStr "feature" matches branches with prefix "feature/".
func FilterByType(worktrees []resolver.WorktreeInfo, ps *resolver.PrefixSet, typeStr string) []resolver.WorktreeInfo {
	target, ok := ps.TypeToPrefix(typeStr)
	if !ok {
		return nil
	}
	prefixes := ps.Strip()
	var out []resolver.WorktreeInfo
	for _, wt := range worktrees {
		_, matchedPrefix := resolver.TaskFromBranch(wt.Branch, prefixes)
		if matchedPrefix == target {
			out = append(out, wt)
		}
	}
	return out
}

// FilterOrphaned splits worktrees into kept and excluded by orphan status.
// No-op (all kept, 0 excluded) when ps.HasCustom() is false.
func FilterOrphaned(worktrees []resolver.WorktreeInfo, ps *resolver.PrefixSet, mainBranch string) (kept []resolver.WorktreeInfo, excluded int) {
	if !ps.HasCustom() {
		return worktrees, 0
	}
	for _, wt := range worktrees {
		if ps.IsOrphan(wt.Branch, mainBranch) {
			excluded++
			continue
		}
		kept = append(kept, wt)
	}
	return kept, excluded
}

// branchDeleteFailedErr builds the unified recovery error for the
// "worktree removed but branch delete failed" partial-failure case.
func branchDeleteFailedErr(branch string, cause error) error {
	return errhint.WithFix(
		fmt.Errorf("worktree removed but failed to delete branch: %w", cause),
		"delete manually: git branch -D "+branch,
	)
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

package operations

import (
	"fmt"

	"github.com/lugassawan/rimba/internal/git"
)

// ArchiveParams holds the inputs for an archive operation.
type ArchiveParams struct {
	Path   string
	Branch string
	Force  bool
	DryRun bool
}

// ArchiveResult holds the outcome of an archive operation.
type ArchiveResult struct {
	Path   string
	Branch string
	Plan   *Plan
}

// ArchiveWorktree removes the worktree directory while preserving the local branch.
func ArchiveWorktree(r git.Runner, params ArchiveParams) (ArchiveResult, error) {
	plan := &Plan{DryRun: params.DryRun}
	result := ArchiveResult{
		Path:   params.Path,
		Branch: params.Branch,
		Plan:   plan,
	}

	desc := fmt.Sprintf("remove worktree: %s (branch %s preserved)", params.Path, params.Branch)
	if err := plan.Do(desc, func() error {
		return git.RemoveWorktree(r, params.Path, params.Force)
	}); err != nil {
		return result, err
	}

	return result, nil
}

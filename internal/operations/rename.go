package operations

import (
	"context"
	"fmt"

	"github.com/lugassawan/rimba/internal/errhint"
	"github.com/lugassawan/rimba/internal/git"
	"github.com/lugassawan/rimba/internal/resolver"
)

// RenameResult holds the outcome of a worktree rename operation.
type RenameResult struct {
	OldBranch string
	NewBranch string
	OldPath   string
	NewPath   string
}

// RenameParams holds the parameters for a worktree rename operation.
// NewPrefix overrides the inherited prefix when non-empty; "" means inherit.
type RenameParams struct {
	WT        resolver.WorktreeInfo
	NewTask   string
	NewPrefix string
	WtDir     string
	Force     bool
}

// RenameWorktree renames a worktree's directory and branch.
// It inherits the prefix from the current branch unless p.NewPrefix is set.
func RenameWorktree(ctx context.Context, r git.Runner, p RenameParams) (RenameResult, error) {
	prefixes := resolver.AllPrefixes()

	svc, _, matchedPrefix := resolver.ServiceFromBranch(p.WT.Branch, prefixes)
	if matchedPrefix == "" {
		matchedPrefix, _ = resolver.PrefixString(resolver.DefaultPrefixType)
	}

	prefix := matchedPrefix
	if p.NewPrefix != "" {
		prefix = p.NewPrefix
	}

	newBranch := resolver.FullBranchName(svc, prefix, p.NewTask)
	if newBranch == p.WT.Branch {
		return RenameResult{}, fmt.Errorf("nothing to change: branch is already %q", p.WT.Branch)
	}

	if git.BranchExists(ctx, r, newBranch) {
		return RenameResult{}, errhint.WithFix(
			fmt.Errorf("branch %q already exists", newBranch),
			"choose a different task name, or remove the existing branch: git branch -D "+newBranch,
		)
	}

	newPath := resolver.WorktreePath(p.WtDir, newBranch)

	if err := git.MoveWorktree(r, p.WT.Path, newPath, p.Force); err != nil {
		return RenameResult{}, errhint.WithFix(
			fmt.Errorf("failed to move worktree: %w", err),
			"unlock the worktree if locked: git worktree unlock "+p.WT.Path+", then retry: rimba rename",
		)
	}

	if err := git.RenameBranch(ctx, r, p.WT.Branch, newBranch); err != nil {
		if rbErr := git.MoveWorktree(r, newPath, p.WT.Path, p.Force); rbErr != nil {
			return RenameResult{}, errhint.WithFix(
				fmt.Errorf("failed to rename branch %q → %q: %w\nRollback failed — worktree is at %s: %w",
					p.WT.Branch, newBranch, err, newPath, rbErr),
				fmt.Sprintf("git worktree move %s %s && git branch -m %s %s",
					newPath, p.WT.Path, p.WT.Branch, newBranch),
			)
		}
		return RenameResult{}, errhint.WithFix(
			fmt.Errorf("failed to rename branch %q → %q: %w\nWorktree moved back to %s",
				p.WT.Branch, newBranch, err, p.WT.Path),
			fmt.Sprintf("retry: git branch -m %s %s", p.WT.Branch, newBranch),
		)
	}

	return RenameResult{
		OldBranch: p.WT.Branch,
		NewBranch: newBranch,
		OldPath:   p.WT.Path,
		NewPath:   newPath,
	}, nil
}

package operations

import (
	"fmt"

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

// RenameWorktree renames a worktree's directory and branch to match a new task name.
// It resolves the new branch name by inheriting the prefix from the current branch.
func RenameWorktree(r git.Runner, wt resolver.WorktreeInfo, newTask, wtDir string, force bool) (RenameResult, error) {
	prefixes := resolver.AllPrefixes()

	svc, _, matchedPrefix := resolver.ServiceFromBranch(wt.Branch, prefixes)
	if matchedPrefix == "" {
		matchedPrefix, _ = resolver.PrefixString(resolver.DefaultPrefixType)
	}

	newBranch := resolver.FullBranchName(svc, matchedPrefix, newTask)

	if git.BranchExists(r, newBranch) {
		return RenameResult{}, fmt.Errorf("branch %q already exists", newBranch)
	}

	newPath := resolver.WorktreePath(wtDir, newBranch)

	if err := git.MoveWorktree(r, wt.Path, newPath, force); err != nil {
		return RenameResult{}, err
	}

	if err := git.RenameBranch(r, wt.Branch, newBranch); err != nil {
		if rbErr := git.MoveWorktree(r, newPath, wt.Path, force); rbErr != nil {
			return RenameResult{}, fmt.Errorf("failed to rename branch %q → %q: %w\nRollback failed — worktree is at %s: %w\nTo recover: git worktree move %s %s && git branch -m %s %s",
				wt.Branch, newBranch, err, newPath, rbErr, newPath, wt.Path, wt.Branch, newBranch)
		}
		return RenameResult{}, fmt.Errorf("failed to rename branch %q → %q: %w\nWorktree moved back to %s\nTo retry: git branch -m %s %s",
			wt.Branch, newBranch, err, wt.Path, wt.Branch, newBranch)
	}

	return RenameResult{
		OldBranch: wt.Branch,
		NewBranch: newBranch,
		OldPath:   wt.Path,
		NewPath:   newPath,
	}, nil
}

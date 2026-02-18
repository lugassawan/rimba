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

	_, matchedPrefix := resolver.TaskFromBranch(wt.Branch, prefixes)
	if matchedPrefix == "" {
		matchedPrefix, _ = resolver.PrefixString(resolver.DefaultPrefixType)
	}

	newBranch := resolver.BranchName(matchedPrefix, newTask)

	if git.BranchExists(r, newBranch) {
		return RenameResult{}, fmt.Errorf("branch %q already exists", newBranch)
	}

	newPath := resolver.WorktreePath(wtDir, newBranch)

	if err := git.MoveWorktree(r, wt.Path, newPath, force); err != nil {
		return RenameResult{}, err
	}

	if err := git.RenameBranch(r, wt.Branch, newBranch); err != nil {
		return RenameResult{}, fmt.Errorf("worktree moved but failed to rename branch %q: %w\nTo complete manually: git branch -m %s %s", wt.Branch, err, wt.Branch, newBranch)
	}

	return RenameResult{
		OldBranch: wt.Branch,
		NewBranch: newBranch,
		OldPath:   wt.Path,
		NewPath:   newPath,
	}, nil
}

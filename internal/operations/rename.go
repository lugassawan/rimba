package operations

import (
	"context"
	"fmt"

	"github.com/lugassawan/rimba/internal/config"
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
	// Push status (only meaningful when Push=true)
	Published      bool  // true if the renamed branch was pushed and set as upstream
	PublishError   error // non-nil if the push failed
	RemoteDeleted  bool  // true if the old remote branch was deleted
	RemoteError    error // non-nil if deleting the old remote branch failed
	RemoteSkipped  bool  // true when there was no upstream to delete (nothing to do)
	NoOriginRemote bool  // true when there is no origin remote configured
}

// RenameParams holds the parameters for a worktree rename operation.
// NewPrefix overrides the inherited prefix when non-empty; "" means inherit.
type RenameParams struct {
	WT        resolver.WorktreeInfo
	NewTask   string
	NewPrefix string
	WtDir     string
	Force     bool
	Push      bool
}

// RenameWorktree renames a worktree's directory and branch.
// It inherits the prefix from the current branch unless p.NewPrefix is set.
func RenameWorktree(ctx context.Context, r git.Runner, p RenameParams) (RenameResult, error) {
	prefixes := config.PrefixSetFromContext(ctx).Strip()

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

	// Must capture before the move: `git branch -m` preserves tracking config, so
	// checking after would still report the stale origin/<old-branch> upstream.
	var hadOriginUpstream bool
	if p.Push {
		upstreamRemote, hasUpstream := git.UpstreamRemote(ctx, r, p.WT.Path)
		hadOriginUpstream = hasUpstream && upstreamRemote == git.DefaultRemote
	}

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

	result := RenameResult{
		OldBranch: p.WT.Branch,
		NewBranch: newBranch,
		OldPath:   p.WT.Path,
		NewPath:   newPath,
	}

	if p.Push {
		publishRenamed(ctx, r, newBranch, newPath, p.WT.Branch, hadOriginUpstream, &result)
	}

	return result, nil
}

// publishRenamed never aborts the rename; outcomes are recorded on result for the
// caller to report. The old remote branch is deleted only after a successful publish
// of the new one — deleting first, or after a failed publish, could leave the remote
// with neither branch.
func publishRenamed(ctx context.Context, r git.Runner, newBranch, newPath, oldBranch string, hadOriginUpstream bool, result *RenameResult) {
	if !git.RemoteExists(ctx, r, git.DefaultRemote) {
		result.NoOriginRemote = true
		return
	}

	if err := git.PushSetUpstream(ctx, r, newPath, git.DefaultRemote, newBranch); err != nil {
		result.PublishError = err
		return
	}
	result.Published = true

	if !hadOriginUpstream {
		result.RemoteSkipped = true
		return
	}

	if err := git.DeleteRemoteBranch(ctx, r, git.DefaultRemote, oldBranch); err != nil {
		result.RemoteError = err
		return
	}
	result.RemoteDeleted = true
}

package operations

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/lugassawan/rimba/internal/errhint"
	"github.com/lugassawan/rimba/internal/git"
	"github.com/lugassawan/rimba/internal/resolver"
)

// PromoteBranch moves the main repo's current branch into its own worktree,
// transferring any dirty working-tree state via git stash push / apply.
// worktreeDir must be an absolute path to the directory that holds worktrees.
func PromoteBranch(ctx context.Context, worktreeDir string, r git.Runner, repoRoot, branch string) (string, error) {
	defaultBranch, err := validateForPromotion(ctx, r, repoRoot, branch)
	if err != nil {
		return "", err
	}

	wtPath := resolver.WorktreePath(worktreeDir, branch)
	if _, err := os.Stat(wtPath); err == nil {
		return "", errhint.WithFix(
			fmt.Errorf("worktree path already exists: %s", wtPath),
			"remove it first or choose a different branch name",
		)
	}

	dirty, err := git.IsDirty(ctx, r, repoRoot)
	if err != nil {
		return "", err
	}
	var stashSHA string
	if dirty {
		stashSHA, err = git.StashPushAndRef(ctx, r, repoRoot, "rimba: promote "+branch)
		if err != nil {
			return "", err
		}
	}

	if err := git.Checkout(ctx, r, repoRoot, defaultBranch); err != nil {
		if restoreErr := restoreStash(ctx, r, repoRoot, stashSHA); restoreErr != nil {
			return "", fmt.Errorf("switch to %s: %w; also failed to restore stash: %w", defaultBranch, err, restoreErr)
		}
		return "", fmt.Errorf("switch to %s: %w", defaultBranch, err)
	}

	if err := git.AddWorktreeFromBranch(ctx, r, wtPath, branch); err != nil {
		// Switch back first while the tree is still clean, then restore the stash.
		// Reversing this order would make the switch fail (dirty tree).
		if switchErr := git.Checkout(ctx, r, repoRoot, branch); switchErr != nil {
			return "", fmt.Errorf("create worktree: %w; also failed to restore HEAD to %s: %w", err, branch, switchErr)
		}
		if restoreErr := restoreStash(ctx, r, repoRoot, stashSHA); restoreErr != nil {
			return "", fmt.Errorf("create worktree: %w; also failed to restore stash: %w", err, restoreErr)
		}
		return "", fmt.Errorf("create worktree: %w", err)
	}

	if stashSHA != "" {
		return wtPath, applyStashToWorktree(ctx, r, wtPath, stashSHA)
	}
	return wtPath, nil
}

// validateForPromotion checks pre-conditions and returns the resolved default branch.
func validateForPromotion(ctx context.Context, r git.Runner, repoRoot, branch string) (string, error) {
	defaultBranch, err := git.DefaultBranch(ctx, r)
	if err != nil {
		return "", err
	}
	if branch == defaultBranch {
		return "", errhint.WithFix(
			fmt.Errorf("cannot promote default branch %q", branch),
			"checkout a feature branch first: git checkout <branch>",
		)
	}
	if !git.BranchExists(ctx, r, branch) {
		return "", errhint.WithFix(
			fmt.Errorf("branch %q does not exist", branch),
			"create the branch first: git checkout -b "+branch,
		)
	}
	entries, err := git.ListWorktrees(ctx, r)
	if err != nil {
		return "", err
	}
	if entry := git.FindEntry(entries, branch); entry != nil && entry.Path != repoRoot {
		return "", errhint.WithFix(
			fmt.Errorf("branch %q is already checked out in worktree %s", branch, entry.Path),
			"use that worktree: cd "+entry.Path,
		)
	}
	current, err := git.CurrentBranch(ctx, r, repoRoot)
	if err != nil {
		return "", err
	}
	if current != branch {
		return "", errhint.WithFix(
			fmt.Errorf("branch %q is not the current branch (HEAD is %q)", branch, current),
			"switch to it first: git checkout "+branch,
		)
	}
	return defaultBranch, nil
}

// restoreStash re-applies and drops a stash entry to undo a failed mid-operation stash push.
// No-ops when sha is empty. Returns an error if the apply fails (stash entry is preserved).
// StashApply and StashDrop are intentionally non-cancellable (recovery paths must complete).
func restoreStash(ctx context.Context, r git.Runner, dir, sha string) error {
	_ = ctx
	if sha == "" {
		return nil
	}
	if err := git.StashApply(r, dir, sha); err != nil {
		return fmt.Errorf("your changes are preserved in stash %s — find it with: git stash list (look for 'rimba: promote ...') then: git stash apply stash@{N}: %w", sha, err)
	}
	if dropErr := git.StashDrop(r, dir, sha); dropErr != nil {
		return fmt.Errorf("stash applied but could not drop entry %s (clean up manually: git stash list, then git stash drop stash@{N}): %w", sha, dropErr)
	}
	return nil
}

// applyStashToWorktree applies a stash to the worktree and drops it on success.
// Any failure preserves the stash so the user can recover: real conflicts get a
// resolve-then-cleanup hint, other apply errors surface their true cause, and a
// failed drop after a clean apply is reported rather than swallowed.
// StashApply and StashDrop are intentionally non-cancellable (recovery paths must complete).
func applyStashToWorktree(ctx context.Context, r git.Runner, wtPath, sha string) error {
	_ = ctx
	if err := git.StashApply(r, wtPath, sha); err != nil {
		if stashApplyConflicted(err) {
			return errhint.WithFix(
				errors.New("stash apply had conflicts"),
				fmt.Sprintf("resolve conflicts in %s (stash SHA: %s), then: git stash list (find 'rimba: promote ...' entry) && git stash drop stash@{N}", wtPath, sha),
			)
		}
		return fmt.Errorf("stash apply failed; your changes are preserved in stash %s — recover them with: cd %s && git stash list (look for 'rimba: promote ...') then git stash apply stash@{N}: %w", sha, wtPath, err)
	}
	if dropErr := git.StashDrop(r, wtPath, sha); dropErr != nil {
		return fmt.Errorf("stash applied but could not drop entry %s (clean up manually: git stash list, then git stash drop stash@{N}): %w", sha, dropErr)
	}
	return nil
}

// stashApplyConflicted reports whether a failed git stash apply was due to merge
// conflicts. git prints CONFLICT markers and StashApply wraps that output, so the
// marker surfaces in the error text (see git.StashApply / git.RunInDir).
func stashApplyConflicted(err error) bool {
	return strings.Contains(err.Error(), "CONFLICT")
}

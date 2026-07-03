package git

import (
	"context"
	"fmt"
	"strings"

	"github.com/lugassawan/rimba/internal/errhint"
)

const (
	cmdWorktree = "worktree"
	flagForce   = "--force"

	// Porcelain output prefixes
	porcelainWorktree = "worktree "
	porcelainHEAD     = "HEAD "
	porcelainBranch   = "branch "
)

// WorktreeEntry represents a parsed worktree from `git worktree list --porcelain`.
type WorktreeEntry struct {
	Path   string
	HEAD   string
	Branch string
	Bare   bool
}

// AddWorktree creates a new worktree at the given path with a new branch from source.
func AddWorktree(ctx context.Context, r Runner, path, branch, source string) error {
	_, err := r.Run(ctx, cmdWorktree, "add", "-b", branch, "--", path, source)
	return err
}

// AddWorktreeFromBranch creates a worktree from an existing branch (no -b flag).
func AddWorktreeFromBranch(ctx context.Context, r Runner, path, branch string) error {
	_, err := r.Run(ctx, cmdWorktree, "add", "--", path, branch)
	return err
}

// RemoveWorktree removes the worktree at the given path.
func RemoveWorktree(ctx context.Context, r Runner, path string, force bool) error {
	args := []string{cmdWorktree, "remove", path}
	if force {
		args = append(args, flagForce)
	}
	_, err := r.Run(ctx, args...)
	return err
}

// MoveWorktree moves the worktree from oldPath to newPath.
// When force is true, --force is passed twice so that even locked worktrees can be moved.
// Intentionally non-cancellable: rollback moves must complete to avoid stranded worktrees.
func MoveWorktree(r Runner, oldPath, newPath string, force bool) error {
	args := []string{cmdWorktree, "move"}
	if force {
		args = append(args, flagForce, flagForce)
	}
	args = append(args, "--", oldPath, newPath)
	_, err := r.Run(context.Background(), args...)
	return err
}

// ListWorktrees returns all worktrees by parsing `git worktree list --porcelain`.
func ListWorktrees(ctx context.Context, r Runner) ([]WorktreeEntry, error) {
	out, err := r.Run(ctx, cmdWorktree, "list", "--porcelain")
	if err != nil {
		return nil, err
	}

	var entries []WorktreeEntry
	var current WorktreeEntry

	for line := range strings.SplitSeq(out, "\n") {
		line = strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(line, porcelainWorktree):
			if current.Path != "" {
				entries = append(entries, current)
			}
			current = WorktreeEntry{Path: strings.TrimPrefix(line, porcelainWorktree)}
		case strings.HasPrefix(line, porcelainHEAD):
			current.HEAD = strings.TrimPrefix(line, porcelainHEAD)
		case strings.HasPrefix(line, porcelainBranch):
			// refs/heads/feat/my-task → feat/my-task
			ref := strings.TrimPrefix(line, porcelainBranch)
			current.Branch = strings.TrimPrefix(ref, refsHeadsPrefix)
		case line == "bare":
			current.Bare = true
		}
	}

	if current.Path != "" {
		entries = append(entries, current)
	}

	return entries, nil
}

// FilterEntries returns entries with bare worktrees, empty branches, and the
// main branch removed. This is the common pre-filter for status, log, and clean.
func FilterEntries(entries []WorktreeEntry, mainBranch string) []WorktreeEntry {
	var out []WorktreeEntry
	for _, e := range entries {
		if e.Bare || e.Branch == "" || e.Branch == mainBranch {
			continue
		}
		out = append(out, e)
	}
	return out
}

// FindEntry returns the first entry matching branch, or nil if none.
// Counterpart to FilterEntries.
func FindEntry(entries []WorktreeEntry, branch string) *WorktreeEntry {
	for i := range entries {
		if entries[i].Branch == branch {
			return &entries[i]
		}
	}
	return nil
}

// Prune runs `git worktree prune` to clean up stale worktree references.
func Prune(ctx context.Context, r Runner, dryRun bool) (string, error) {
	args := []string{cmdWorktree, "prune"}
	if dryRun {
		args = append(args, "--dry-run")
	}
	out, err := r.Run(ctx, args...)
	if err != nil {
		return "", errhint.WithFix(
			fmt.Errorf("prune: %w", err),
			"check repo permissions, then run: git worktree list",
		)
	}
	return out, nil
}

package git

import (
	"fmt"
	"strings"
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
func AddWorktree(r Runner, path, branch, source string) error {
	_, err := r.Run(cmdWorktree, "add", "-b", branch, path, source)
	return err
}

// AddWorktreeFromBranch creates a worktree from an existing branch (no -b flag).
func AddWorktreeFromBranch(r Runner, path, branch string) error {
	_, err := r.Run(cmdWorktree, "add", path, branch)
	return err
}

// RemoveWorktree removes the worktree at the given path.
func RemoveWorktree(r Runner, path string, force bool) error {
	args := []string{cmdWorktree, "remove", path}
	if force {
		args = append(args, flagForce)
	}
	_, err := r.Run(args...)
	return err
}

// MoveWorktree moves the worktree from oldPath to newPath.
// When force is true, --force is passed twice so that even locked worktrees can be moved.
func MoveWorktree(r Runner, oldPath, newPath string, force bool) error {
	args := []string{cmdWorktree, "move", oldPath, newPath}
	if force {
		args = append(args, flagForce, flagForce)
	}
	_, err := r.Run(args...)
	return err
}

// ListWorktrees returns all worktrees by parsing `git worktree list --porcelain`.
func ListWorktrees(r Runner) ([]WorktreeEntry, error) {
	out, err := r.Run(cmdWorktree, "list", "--porcelain")
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

// Prune runs `git worktree prune` to clean up stale worktree references.
func Prune(r Runner, dryRun bool) (string, error) {
	args := []string{cmdWorktree, "prune"}
	if dryRun {
		args = append(args, "--dry-run")
	}
	out, err := r.Run(args...)
	if err != nil {
		return "", fmt.Errorf("prune: %w", err)
	}
	return out, nil
}

package operations

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/lugassawan/rimba/internal/errhint"
	"github.com/lugassawan/rimba/internal/git"
)

// LockInfo describes a stale git index.lock file found under a worktree's
// admin directory in <commonDir>/worktrees/<id>/index.lock.
type LockInfo struct {
	Path    string
	ModTime time.Time
	Age     time.Duration
}

// LockRemoval records the outcome of removing one stale lock.
type LockRemoval struct {
	Path string
	Err  error
}

// MinLockAge is the minimum age before automated recovery touches a lock —
// younger locks may still belong to a running git process.
const MinLockAge = 10 * time.Second

// ScanWorktreeLocks finds index.lock files left behind under
// <commonDir>/worktrees/*/index.lock. These linger when a killed git
// process is SIGKILLed before it can run its own atexit cleanup (#380).
func ScanWorktreeLocks(commonDir string) ([]LockInfo, error) {
	matches, err := filepath.Glob(filepath.Join(commonDir, "worktrees", "*", "index.lock"))
	if err != nil {
		return nil, errhint.WithFix(
			fmt.Errorf("scan for stale locks under %s: %w", commonDir, err),
			"the repository path may contain characters glob treats specially (e.g. an unmatched '['); rename it and retry",
		)
	}

	now := time.Now()
	locks := make([]LockInfo, 0, len(matches))
	for _, path := range matches {
		info, err := os.Stat(path)
		if err != nil {
			continue
		}
		locks = append(locks, LockInfo{
			Path:    path,
			ModTime: info.ModTime(),
			Age:     now.Sub(info.ModTime()),
		})
	}
	return locks, nil
}

// RemoveStaleLocks deletes each lock file, capturing per-path errors so one
// failure (e.g. a permissions issue) doesn't abort the rest.
func RemoveStaleLocks(locks []LockInfo) []LockRemoval {
	removals := make([]LockRemoval, 0, len(locks))
	for _, l := range locks {
		removals = append(removals, LockRemoval{Path: l.Path, Err: os.Remove(l.Path)})
	}
	return removals
}

// InterruptedWorktree describes a worktree left in an "interrupted clean"
// state: a killed `git worktree remove` deleted its tracked files but never
// got to deregister the worktree or touch the index, so every entry in
// `git status --porcelain` shows up as an unstaged deletion.
type InterruptedWorktree struct {
	Path         string
	Branch       string
	DeletedCount int
}

// ScanInterruptedWorktrees lists all worktrees and reports the ones that look
// interrupted-clean. Per-worktree status errors are swallowed (treated as
// not-interrupted) so one bad worktree can't abort the whole scan — only a
// ListWorktrees failure propagates as a hard error.
func ScanInterruptedWorktrees(ctx context.Context, r git.Runner) ([]InterruptedWorktree, error) {
	entries, err := git.ListWorktrees(ctx, r)
	if err != nil {
		return nil, err
	}

	var interrupted []InterruptedWorktree
	for _, e := range entries {
		if iw, ok := classifyInterruptedWorktree(ctx, r, e); ok {
			interrupted = append(interrupted, iw)
		}
	}
	return interrupted, nil
}

// classifyInterruptedWorktree reports whether a single worktree entry looks
// interrupted-clean. Bare and prunable worktrees are skipped outright; a
// missing/malformed .git pointer file (the main worktree's `.git` is a
// directory, not a pointer file) excludes it for free; a present
// index.lock defers to the lock-scan flow instead. Any git status error is
// swallowed — treated as not-interrupted.
func classifyInterruptedWorktree(ctx context.Context, r git.Runner, e git.WorktreeEntry) (InterruptedWorktree, bool) {
	if e.Bare || e.Prunable {
		return InterruptedWorktree{}, false
	}

	adminDir, ok := readWorktreeAdminDir(e.Path)
	if !ok {
		return InterruptedWorktree{}, false
	}

	if hasIndexLock(adminDir) {
		return InterruptedWorktree{}, false
	}

	status, err := git.ClassifyPorcelainDeletions(ctx, r, e.Path)
	if err != nil {
		return InterruptedWorktree{}, false
	}
	if !status.AllDeletions() {
		return InterruptedWorktree{}, false
	}

	return InterruptedWorktree{Path: e.Path, Branch: e.Branch, DeletedCount: status.Deleted}, true
}

// hasIndexLock reports whether adminDir has an index.lock file, treating any
// stat error (including "not found") as "no lock present" — the same
// permissive style ScanWorktreeLocks uses.
func hasIndexLock(adminDir string) bool {
	_, err := os.Stat(filepath.Join(adminDir, "index.lock"))
	return err == nil
}

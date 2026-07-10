package operations

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/lugassawan/rimba/internal/errhint"
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

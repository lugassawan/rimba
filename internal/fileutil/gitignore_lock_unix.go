//go:build !windows

package fileutil

import (
	"errors"
	"os"
	"syscall"
)

// gitignoreLockOwnerAlive reports whether pid is still running, using a
// signal-0 liveness probe. Unix-only: Windows has no equivalent, so
// stale-lock reclaim there relies on the age fallback in
// reclaimStaleGitignoreLock instead.
//
// Only a confirmed-dead signal (ESRCH, or Go's own ErrProcessDone for a pid
// it already reaped) counts as dead. Other errors like EPERM (process
// exists but owned by another user) don't prove the process is gone, so
// they're treated as alive.
func gitignoreLockOwnerAlive(pid int) bool {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	err = proc.Signal(syscall.Signal(0))
	if err == nil {
		return true
	}
	return !errors.Is(err, syscall.ESRCH) && !errors.Is(err, os.ErrProcessDone)
}

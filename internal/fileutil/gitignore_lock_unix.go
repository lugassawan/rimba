//go:build !windows

package fileutil

import (
	"os"
	"syscall"
)

// gitignoreLockOwnerAlive reports whether pid is still running, using a
// signal-0 liveness probe. Unix-only: Windows has no equivalent primitive,
// so stale-lock reclaim there relies solely on the age fallback in
// reclaimStaleGitignoreLock.
func gitignoreLockOwnerAlive(pid int) bool {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	return proc.Signal(syscall.Signal(0)) == nil
}

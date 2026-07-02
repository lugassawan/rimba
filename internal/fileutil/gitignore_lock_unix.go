//go:build !windows

package fileutil

import (
	"errors"
	"os"
	"syscall"
)

// gitignoreLockOwnerAlive uses a signal-0 liveness probe (Unix-only; Windows relies on the age fallback instead).
// Only ESRCH or Go's ErrProcessDone count as confirmed-dead; other errors like EPERM are treated as alive.
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

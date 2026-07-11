//go:build !windows

package proc

import (
	"errors"
	"os"
	"syscall"
)

// Alive reports whether pid is still running, using a signal-0 probe.
// Only ESRCH/ErrProcessDone count as dead; other errors are treated as alive.
func Alive(pid int) bool {
	p, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	err = p.Signal(syscall.Signal(0))
	if err == nil {
		return true
	}
	return !errors.Is(err, syscall.ESRCH) && !errors.Is(err, os.ErrProcessDone)
}

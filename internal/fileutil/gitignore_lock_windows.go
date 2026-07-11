//go:build windows

package fileutil

import "github.com/lugassawan/rimba/internal/proc"

// gitignoreLockOwnerAlive always reports true on Windows: there's no signal-0 probe here, so reclaim relies solely on the age fallback.
func gitignoreLockOwnerAlive(pid int) bool {
	return proc.Alive(pid)
}

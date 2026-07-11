//go:build !windows

package fileutil

import "github.com/lugassawan/rimba/internal/proc"

// gitignoreLockOwnerAlive delegates to proc.Alive (Unix-only; Windows relies on the age fallback instead).
func gitignoreLockOwnerAlive(pid int) bool {
	return proc.Alive(pid)
}

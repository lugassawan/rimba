//go:build windows

package updater

import (
	"os"
	"path/filepath"
)

// SweepOldBinary removes the exe.old file left by a previous rename-aside
// swap.  Called at startup so the binary is not the one being deleted.
// Best-effort: errors are swallowed.
func SweepOldBinary() {
	exe, err := os.Executable()
	if err != nil {
		return
	}
	resolved, err := filepath.EvalSymlinks(exe)
	if err != nil {
		return
	}
	sweepOldBinary(resolved)
}

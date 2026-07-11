//go:build !windows

package operations

import (
	"os"
	"syscall"
)

// dirIno returns path's inode number, a stable identity across an internal
// mtime bump (e.g. a lock file written inside the directory).
func dirIno(path string) (ino uint64, ok bool) {
	info, err := os.Stat(path)
	if err != nil {
		return 0, false
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return 0, false
	}
	return stat.Ino, true
}

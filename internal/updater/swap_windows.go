//go:build windows

package updater

// swapBinary replaces dst with the contents of tmpPath using the rename-aside
// dance: dst is moved to dst.old (Windows permits renaming a running binary),
// then tmpPath is copied into dst.  The caller's defer removes tmpPath.
func swapBinary(tmpPath, dst string) error {
	return renameAside(tmpPath, dst)
}

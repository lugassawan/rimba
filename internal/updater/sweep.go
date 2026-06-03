package updater

import "os"

// sweepOldBinary removes the leftover exePath.old file left by a prior
// rename-aside swap.  Errors are swallowed: a running binary cannot delete
// itself, so the file may legitimately be locked on Windows.
func sweepOldBinary(exePath string) {
	_ = os.Remove(exePath + oldBinarySuffix)
}

//go:build !windows

package updater

import "os"

// swapBinary atomically replaces dst with tmpPath via os.Rename.
// On Unix, Rename is guaranteed atomic within the same filesystem.
func swapBinary(tmpPath, dst string) error {
	return os.Rename(tmpPath, dst) //nolint:gosec // dst is the resolved current binary path, not user input
}

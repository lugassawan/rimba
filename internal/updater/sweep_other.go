//go:build !windows

package updater

// SweepOldBinary is a no-op on non-Windows platforms.
func SweepOldBinary() {}

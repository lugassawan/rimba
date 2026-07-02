//go:build windows

package fileutil

// gitignoreLockOwnerAlive always reports true on Windows: there's no signal-0 probe here, so reclaim relies solely on the age fallback.
func gitignoreLockOwnerAlive(int) bool {
	return true
}

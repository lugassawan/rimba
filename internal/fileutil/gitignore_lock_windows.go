//go:build windows

package fileutil

// gitignoreLockOwnerAlive always reports true on Windows: there is no
// reliable signal-0 liveness probe here, so stale-lock reclaim relies
// solely on the age fallback in reclaimStaleGitignoreLock.
func gitignoreLockOwnerAlive(int) bool {
	return true
}

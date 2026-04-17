// Package fsutil provides filesystem utilities that are shared across
// rimba commands (directory sizing, path helpers, etc.).
package fsutil

import (
	"io/fs"
	"path/filepath"
)

// DirSize walks path and returns the total byte size of regular files
// reachable without traversing symlinks. On per-entry errors (permission
// denied, races) it accumulates a best-effort total and returns the first
// error observed. The returned size is meaningful even when err is non-nil.
func DirSize(path string) (int64, error) {
	var total int64
	var firstErr error

	record := func(err error) {
		if firstErr == nil {
			firstErr = err
		}
	}

	walkErr := filepath.WalkDir(path, func(_ string, d fs.DirEntry, err error) error {
		if err != nil {
			record(err)
			if d != nil && d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if !d.Type().IsRegular() {
			return nil
		}
		info, infoErr := d.Info()
		if infoErr != nil {
			record(infoErr)
			return nil
		}
		total += info.Size()
		return nil
	})
	if walkErr != nil {
		record(walkErr)
	}
	return total, firstErr
}

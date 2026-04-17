// Package fsutil holds filesystem helpers shared across commands.
package fsutil

import (
	"io/fs"
	"path/filepath"
)

// DirSize returns the total size of regular files under path.
// Symlinks are not followed. On partial failure (permission denied,
// races) it returns the best-effort total and the first error seen.
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

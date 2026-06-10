// Package fsutil holds filesystem helpers shared across commands.
package fsutil

import (
	"context"
	"io/fs"
	"path/filepath"
)

type dirSizeResult struct {
	size int64
	err  error
}

// DirSize returns the total size of regular files under path.
// Symlinks are not followed. On partial failure (permission denied,
// races) it returns the best-effort total and the first error seen.
//
// If ctx is cancelled before the walk completes, DirSize returns (0, ctx.Err()).
// The underlying WalkDir runs in a goroutine so a stuck syscall (e.g. stalled
// NFS mount) cannot block the caller beyond the context deadline.
func DirSize(ctx context.Context, path string) (int64, error) {
	ch := make(chan dirSizeResult, 1)
	go func() {
		size, err := walkDirSize(path)
		ch <- dirSizeResult{size: size, err: err}
	}()

	select {
	case <-ctx.Done():
		return 0, ctx.Err()
	case r := <-ch:
		return r.size, r.err
	}
}

func walkDirSize(path string) (int64, error) {
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

// Package fsutil holds filesystem helpers shared across commands.
package fsutil

import (
	"context"
	"io/fs"
	"path/filepath"
	"runtime/debug"

	"github.com/lugassawan/rimba/internal/parallel"
)

type dirSizeResult struct {
	size     int64
	err      error
	panicVal any    // non-nil when the walker panicked
	stack    []byte // walker stack captured at the panic
}

// DirSize returns the total size of regular files under path.
// Symlinks are not followed; partial failures return a best-effort total.
//
// Returns (0, ctx.Err()) immediately on cancellation. The WalkDir goroutine
// continues until the OS returns — it cannot be interrupted mid-syscall.
// A walker panic is re-raised on the caller's goroutine (dropped if ctx won).
func DirSize(ctx context.Context, path string) (int64, error) {
	return dirSizeWith(ctx, path, walkDirSize)
}

func dirSizeWith(ctx context.Context, path string, walk func(string) (int64, error)) (int64, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	ch := make(chan dirSizeResult, 1)
	go func() {
		defer func() {
			if p := recover(); p != nil {
				ch <- dirSizeResult{panicVal: p, stack: debug.Stack()}
			}
		}()
		size, err := walk(path)
		ch <- dirSizeResult{size: size, err: err}
	}()

	select {
	case <-ctx.Done():
		return 0, ctx.Err()
	case r := <-ch:
		if r.panicVal != nil {
			parallel.ReportPanic(r.panicVal, r.stack)
			panic(r.panicVal)
		}
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

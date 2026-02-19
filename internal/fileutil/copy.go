package fileutil

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

const copyErrFmt = "copy %s: %w"

// CopyEntries copies the listed files or directories from src directory to dst directory.
// Missing source entries are silently skipped. Returns the list of entries actually copied.
func CopyEntries(src, dst string, entries []string) ([]string, error) {
	copied := make([]string, 0, len(entries))
	for _, name := range entries {
		srcPath := filepath.Join(src, name)
		dstPath := filepath.Join(dst, name)

		info, err := os.Stat(srcPath)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return copied, fmt.Errorf(copyErrFmt, name, err)
		}

		if info.IsDir() {
			if err := copyDir(srcPath, dstPath); err != nil {
				return copied, fmt.Errorf(copyErrFmt, name, err)
			}
		} else {
			if err := os.MkdirAll(filepath.Dir(dstPath), 0750); err != nil {
				return copied, fmt.Errorf(copyErrFmt, name, err)
			}
			if err := copyFile(srcPath, dstPath); err != nil {
				return copied, fmt.Errorf(copyErrFmt, name, err)
			}
		}
		copied = append(copied, name)
	}
	return copied, nil
}

func copyDir(src, dst string) error {
	srcInfo, err := os.Stat(src)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dst, srcInfo.Mode().Perm()); err != nil {
		return err
	}
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())
		if entry.Type()&os.ModeSymlink != 0 {
			continue // skip symlinks
		}
		if entry.IsDir() {
			if err := copyDir(srcPath, dstPath); err != nil {
				return err
			}
		} else {
			if err := copyFile(srcPath, dstPath); err != nil {
				return err
			}
		}
	}
	return nil
}

// SkippedEntries returns the entries from requested that are not in copied.
func SkippedEntries(requested, copied []string) []string {
	set := make(map[string]struct{}, len(copied))
	for _, c := range copied {
		set[c] = struct{}{}
	}
	var skipped []string
	for _, r := range requested {
		if _, ok := set[r]; !ok {
			skipped = append(skipped, r)
		}
	}
	return skipped
}

func copyFile(src, dst string) (retErr error) {
	in, err := os.Open(filepath.Clean(src))
	if err != nil {
		return err
	}
	defer func() { _ = in.Close() }()

	info, err := in.Stat()
	if err != nil {
		return err
	}

	out, err := os.OpenFile(filepath.Clean(dst), os.O_WRONLY|os.O_CREATE|os.O_TRUNC, info.Mode()) //nolint:gosec // dst is derived from config copy_files, not user input
	if err != nil {
		return err
	}
	defer func() {
		if cerr := out.Close(); retErr == nil {
			retErr = cerr
		}
	}()

	_, err = io.Copy(out, in)
	return err
}

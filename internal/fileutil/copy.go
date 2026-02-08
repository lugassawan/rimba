package fileutil

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// CopyDotfiles copies the listed files from src directory to dst directory.
// Missing source files are silently skipped. Returns the list of files actually copied.
func CopyDotfiles(src, dst string, files []string) ([]string, error) {
	copied := make([]string, 0, len(files))
	for _, name := range files {
		srcPath := filepath.Join(src, name)
		dstPath := filepath.Join(dst, name)

		if err := copyFile(srcPath, dstPath); err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return copied, fmt.Errorf("copy %s: %w", name, err)
		}
		copied = append(copied, name)
	}
	return copied, nil
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

	out, err := os.OpenFile(filepath.Clean(dst), os.O_WRONLY|os.O_CREATE|os.O_TRUNC, info.Mode())
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

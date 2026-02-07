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
	var copied []string
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

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	info, err := in.Stat()
	if err != nil {
		return err
	}

	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, info.Mode())
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}

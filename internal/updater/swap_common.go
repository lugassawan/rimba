package updater

import (
	"fmt"
	"io"
	"os"
)

const oldBinarySuffix = ".old"

// renameAside installs tmpPath at dst: moves dst → dst.old, copies tmpPath → dst,
// then chmods to match.  Rolls back (dst.old → dst) on failure; surfaces both the
// copy error and the rollback error if restore also fails.
// The caller must pre-set tmpPath permissions; renameAside reads them via Stat.
func renameAside(tmpPath, dst string) error {
	info, err := os.Stat(tmpPath)
	if err != nil {
		return fmt.Errorf("stat new binary: %w", err)
	}
	perm := info.Mode().Perm()

	old := dst + oldBinarySuffix
	if err := os.Rename(dst, old); err != nil {
		return fmt.Errorf("moving aside old binary: %w", err)
	}

	if err := copyFile(tmpPath, dst, perm); err != nil {
		rmErr := os.Remove(dst) // clear any partial write; on Windows Rename fails if dst exists
		if rbErr := os.Rename(old, dst); rbErr != nil {
			return fmt.Errorf("installing new binary: %w; rollback failed (remove: %w, rename: %w) — original binary is at %s", err, rmErr, rbErr, old)
		}
		return fmt.Errorf("installing new binary: %w", err)
	}

	return nil
}

func copyFile(src, dst string, perm os.FileMode) error {
	in, err := os.Open(src) //nolint:gosec // src is the verified temp file path, not user input
	if err != nil {
		return fmt.Errorf("opening source: %w", err)
	}

	// Create with narrow permissions; Chmod below sets the final mode without
	// relying on OpenFile's umask-adjusted result.
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600) //nolint:gosec // dst is the resolved current binary path, not user input
	if err != nil {
		_ = in.Close()
		return fmt.Errorf("creating destination: %w", err)
	}

	if _, err := io.Copy(out, in); err != nil {
		_ = in.Close()
		_ = out.Close()
		return fmt.Errorf("writing: %w", err)
	}
	_ = in.Close()

	if err := out.Close(); err != nil {
		return fmt.Errorf("closing destination: %w", err)
	}

	if err := os.Chmod(dst, perm); err != nil { //nolint:gosec // dst is the resolved current binary path, not user input
		return fmt.Errorf("setting permissions: %w", err)
	}

	return nil
}

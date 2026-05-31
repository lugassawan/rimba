package updater

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// dispatchExtract selects tar.gz or zip extraction based on the archive path extension
// and writes the platform binary (rimba or rimba.exe) to dstDir.
func dispatchExtract(archivePath, dstDir string, isWindows bool) (string, error) {
	binName := binaryName
	if isWindows {
		binName = binaryName + ".exe"
	}
	if strings.HasSuffix(archivePath, ".zip") {
		return extractZip(archivePath, dstDir, binName)
	}
	return extractTarGz(archivePath, dstDir, binName)
}

// extractTarGz opens a .tar.gz archive file and extracts binName to dstDir.
func extractTarGz(archivePath, dstDir, binName string) (string, error) {
	f, err := os.Open(filepath.Clean(archivePath))
	if err != nil {
		return "", fmt.Errorf("opening archive: %w", err)
	}
	defer func() { _ = f.Close() }()

	gz, err := gzip.NewReader(f)
	if err != nil {
		return "", fmt.Errorf("decompressing archive: %w", err)
	}
	defer func() { _ = gz.Close() }()

	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", fmt.Errorf("reading archive: %w", err)
		}
		if hdr.Typeflag != tar.TypeReg {
			continue
		}
		if hdr.Name == binName {
			return writeBinary(filepath.Join(dstDir, binName), tr)
		}
	}
	return "", fmt.Errorf("binary %q not found in archive", binName)
}

// extractZip opens a .zip archive file and extracts binName to dstDir.
func extractZip(archivePath, dstDir, binName string) (string, error) {
	zr, err := zip.OpenReader(archivePath)
	if err != nil {
		return "", fmt.Errorf("opening zip archive: %w", err)
	}
	defer func() { _ = zr.Close() }()

	for _, f := range zr.File {
		if f.Name != binName {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return "", fmt.Errorf("opening zip entry: %w", err)
		}
		dst, writeErr := writeBinary(filepath.Join(dstDir, binName), rc)
		_ = rc.Close()
		return dst, writeErr
	}
	return "", fmt.Errorf("binary %q not found in archive", binName)
}

// writeBinary writes the contents of r to dst with executable permissions.
// It enforces maxBinarySize to guard against zip-bomb payloads.
func writeBinary(dst string, r io.Reader) (_ string, retErr error) {
	f, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY, 0755) //nolint:gosec // executable binary requires 0755
	if err != nil {
		return "", fmt.Errorf("creating binary file: %w", err)
	}
	defer func() {
		if cerr := f.Close(); retErr == nil {
			retErr = cerr
		}
	}()

	lr := io.LimitReader(r, maxBinarySize+1)
	n, err := io.Copy(f, lr)
	if err != nil {
		return "", fmt.Errorf("extracting binary: %w", err)
	}
	if n > maxBinarySize {
		_ = os.Remove(dst)
		return "", fmt.Errorf("binary exceeds max allowed size of %d bytes", maxBinarySize)
	}
	return dst, nil
}

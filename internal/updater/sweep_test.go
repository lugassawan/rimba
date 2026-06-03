package updater

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSweepOldBinaryRemovesOld(t *testing.T) {
	tmpDir := t.TempDir()
	exe := filepath.Join(tmpDir, "rimba")
	old := exe + oldBinarySuffix

	if err := os.WriteFile(old, []byte("stale"), 0755); err != nil {
		t.Fatal(err)
	}

	sweepOldBinary(exe)

	if _, err := os.Stat(old); !os.IsNotExist(err) {
		t.Error("expected exe.old to be removed")
	}
}

func TestSweepOldBinaryNoOldFile(t *testing.T) {
	tmpDir := t.TempDir()
	exe := filepath.Join(tmpDir, "rimba")

	// No exe.old present — best-effort sweep should not panic.
	sweepOldBinary(exe)
}

func TestSweepOldBinaryExported(t *testing.T) {
	// SweepOldBinary is a no-op on non-Windows; verify it does not panic.
	SweepOldBinary()
}

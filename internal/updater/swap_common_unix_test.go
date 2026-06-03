//go:build !windows

package updater

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"testing"
	"time"
)

// TestRenameAsideCompoundRollbackFail covers the double-failure branch in
// renameAside: copyFile fails AND the rollback Rename also fails.
//
// A named pipe (FIFO) is used as tmpPath so that os.Open inside copyFile
// blocks until the goroutine has (a) confirmed the initial rename succeeded
// by detecting dst.old's appearance, and (b) made binDir read-only — ensuring
// the directory is non-writable before the copy is attempted, with no race.
func TestRenameAsideCompoundRollbackFail(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("chmod 0555 has no effect as root")
	}

	binDir := t.TempDir()
	dst := filepath.Join(binDir, "binary")
	old := dst + oldBinarySuffix

	if err := os.WriteFile(dst, []byte("original"), 0755); err != nil {
		t.Fatal(err)
	}

	fifoDir := t.TempDir()
	fifoPath := filepath.Join(fifoDir, "tmpbin")
	if err := syscall.Mkfifo(fifoPath, 0644); err != nil {
		t.Fatalf("mkfifo: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(binDir, 0755) })

	go func() {
		// Poll until renameAside has moved dst → old, then make binDir
		// read-only so that both the copy attempt and the rollback rename fail.
		deadline := time.Now().Add(5 * time.Second)
		for time.Now().Before(deadline) {
			if _, err := os.Stat(old); err == nil {
				break
			}
			runtime.Gosched()
		}
		_ = os.Chmod(binDir, 0555)
		// Opening the FIFO write end unblocks os.Open(fifoPath) in copyFile;
		// binDir is already read-only by this point.
		if w, err := os.OpenFile(fifoPath, os.O_WRONLY, 0); err == nil {
			_ = w.Close()
		}
	}()

	err := renameAside(fifoPath, dst)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "rollback failed") {
		t.Errorf("error = %q, want to contain 'rollback failed'", err.Error())
	}
}

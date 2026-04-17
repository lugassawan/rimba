package fsutil_test

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/lugassawan/rimba/internal/fsutil"
)

func TestDirSizeSumsRegularFiles(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "a.txt"), 100)
	writeFile(t, filepath.Join(root, "nested", "b.txt"), 250)
	writeFile(t, filepath.Join(root, "nested", "deeper", "c.txt"), 50)

	got, err := fsutil.DirSize(root)
	if err != nil {
		t.Fatalf("DirSize returned error: %v", err)
	}
	if want := int64(400); got != want {
		t.Errorf("DirSize = %d, want %d", got, want)
	}
}

func TestDirSizeDoesNotFollowSymlinks(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlinks require elevated privileges on Windows")
	}

	outside := t.TempDir()
	writeFile(t, filepath.Join(outside, "huge.bin"), 10_000)

	root := t.TempDir()
	writeFile(t, filepath.Join(root, "small.txt"), 42)
	if err := os.Symlink(filepath.Join(outside, "huge.bin"), filepath.Join(root, "link")); err != nil {
		t.Fatalf("symlink: %v", err)
	}

	got, err := fsutil.DirSize(root)
	if err != nil {
		t.Fatalf("DirSize returned error: %v", err)
	}
	if got >= 10_000 {
		t.Errorf("DirSize = %d, symlink target appears to have been followed (expected small count)", got)
	}
	if got < 42 {
		t.Errorf("DirSize = %d, expected at least the small.txt byte count (42)", got)
	}
}

func TestDirSizeEmptyDir(t *testing.T) {
	root := t.TempDir()
	got, err := fsutil.DirSize(root)
	if err != nil {
		t.Fatalf("DirSize returned error: %v", err)
	}
	if got != 0 {
		t.Errorf("DirSize = %d, want 0 for empty dir", got)
	}
}

func TestDirSizeMissingPath(t *testing.T) {
	_, err := fsutil.DirSize(filepath.Join(t.TempDir(), "does-not-exist"))
	if err == nil {
		t.Fatal("DirSize on missing path returned nil error, want non-nil")
	}
}

func TestDirSizeBestEffortOnPartialError(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("chmod-based permission test is POSIX-specific")
	}
	if os.Geteuid() == 0 {
		t.Skip("running as root bypasses permission checks")
	}

	root := t.TempDir()
	writeFile(t, filepath.Join(root, "visible.txt"), 123)

	denied := filepath.Join(root, "denied")
	if err := os.Mkdir(denied, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	writeFile(t, filepath.Join(denied, "secret.txt"), 999)
	if err := os.Chmod(denied, 0o000); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(denied, 0o755) })

	got, err := fsutil.DirSize(root)
	if err == nil {
		t.Fatal("DirSize returned nil error despite permission-denied subdir")
	}
	if !strings.Contains(err.Error(), "denied") && !strings.Contains(err.Error(), "permission") {
		t.Logf("DirSize error = %v (ok — non-nil)", err)
	}
	if got < 123 {
		t.Errorf("DirSize = %d, want at least 123 (best-effort should include visible.txt)", got)
	}
}

func writeFile(t *testing.T, path string, size int) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	buf := make([]byte, size)
	for i := range buf {
		buf[i] = 'x'
	}
	if err := os.WriteFile(path, buf, 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

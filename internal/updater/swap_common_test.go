package updater

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRenameAsideSuccess(t *testing.T) {
	tmpDir := t.TempDir()

	dst := filepath.Join(tmpDir, "binary")
	if err := os.WriteFile(dst, []byte("old content"), 0755); err != nil {
		t.Fatal(err)
	}

	src := filepath.Join(tmpDir, "new")
	if err := os.WriteFile(src, []byte("new content"), 0755); err != nil {
		t.Fatal(err)
	}

	if err := renameAside(src, dst); err != nil {
		t.Fatalf("renameAside: %v", err)
	}

	got, err := os.ReadFile(dst)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "new content" {
		t.Errorf("dst content = %q, want %q", got, "new content")
	}

	old, err := os.ReadFile(dst + oldBinarySuffix)
	if err != nil {
		t.Fatal(err)
	}
	if string(old) != "old content" {
		t.Errorf("dst.old content = %q, want %q", old, "old content")
	}

	info, err := os.Stat(dst)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0755 {
		t.Errorf("dst perm = %04o, want 0755", info.Mode().Perm())
	}
}

func TestRenameAsideRollbackOnCopyError(t *testing.T) {
	tmpDir := t.TempDir()

	dst := filepath.Join(tmpDir, "binary")
	if err := os.WriteFile(dst, []byte("original"), 0755); err != nil {
		t.Fatal(err)
	}

	// A directory as src: os.Open succeeds but io.Copy returns EISDIR.
	srcDir := t.TempDir()

	err := renameAside(srcDir, dst)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "installing new binary") {
		t.Errorf("error = %q, want to contain 'installing new binary'", err.Error())
	}

	got, err := os.ReadFile(dst)
	if err != nil {
		t.Fatalf("dst should be restored after rollback: %v", err)
	}
	if string(got) != "original" {
		t.Errorf("dst content after rollback = %q, want %q", got, "original")
	}

	if _, err := os.Stat(dst + oldBinarySuffix); !os.IsNotExist(err) {
		t.Error("dst.old should not exist after successful rollback")
	}
}

func TestRenameAsideStatError(t *testing.T) {
	tmpDir := t.TempDir()

	dst := filepath.Join(tmpDir, "binary")
	if err := os.WriteFile(dst, []byte("original"), 0755); err != nil {
		t.Fatal(err)
	}

	nonexistent := filepath.Join(tmpDir, "nonexistent_new")
	err := renameAside(nonexistent, dst)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "stat new binary") {
		t.Errorf("error = %q, want to contain 'stat new binary'", err.Error())
	}

	got, readErr := os.ReadFile(dst)
	if readErr != nil || string(got) != "original" {
		t.Error("dst should be untouched after stat failure")
	}
}

func TestRenameAsideMoveAsideError(t *testing.T) {
	tmpDir := t.TempDir()

	src := filepath.Join(tmpDir, "new")
	if err := os.WriteFile(src, []byte("new content"), 0755); err != nil {
		t.Fatal(err)
	}

	// dst does not exist: os.Rename fails immediately, nothing is moved.
	nonexistent := filepath.Join(tmpDir, "does_not_exist")

	err := renameAside(src, nonexistent)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "moving aside old binary") {
		t.Errorf("error = %q, want to contain 'moving aside old binary'", err.Error())
	}

	if _, err := os.Stat(nonexistent + oldBinarySuffix); !os.IsNotExist(err) {
		t.Error("dst.old should not exist when move-aside fails")
	}
}

// TestRenameAsideRollbackDoubleFailSubcomponents confirms that copy failure and rollback
// Rename failure can both occur in the same directory context, exercising the sub-errors
// that feed into renameAside's compound "rollback failed" message.
func TestRenameAsideRollbackDoubleFailSubcomponents(t *testing.T) {
	binDir := t.TempDir()
	dst := filepath.Join(binDir, "binary")
	old := dst + oldBinarySuffix

	if err := os.WriteFile(dst, []byte("original"), 0755); err != nil {
		t.Fatal(err)
	}

	// Simulate the move-aside that renameAside performs before the copy.
	if err := os.Rename(dst, old); err != nil {
		t.Fatal("setup move-aside:", err)
	}

	if err := os.Chmod(binDir, 0555); err != nil {
		t.Fatal("chmod:", err)
	}
	t.Cleanup(func() { _ = os.Chmod(binDir, 0755) })

	src := filepath.Join(t.TempDir(), "src")
	if err := os.WriteFile(src, []byte("new content"), 0755); err != nil {
		t.Fatal(err)
	}

	copyErr := copyFile(src, dst, 0755)
	if copyErr == nil {
		t.Fatal("expected copyFile to fail in read-only dir")
	}
	if !strings.Contains(copyErr.Error(), "creating destination") {
		t.Errorf("copyErr = %q, want to contain 'creating destination'", copyErr.Error())
	}

	rbErr := os.Rename(old, dst)
	if rbErr == nil {
		t.Fatal("expected rollback Rename to fail in read-only dir")
	}
}

func TestCopyFileMissingSource(t *testing.T) {
	tmpDir := t.TempDir()
	src := filepath.Join(tmpDir, "nonexistent")
	dst := filepath.Join(tmpDir, "dst")

	err := copyFile(src, dst, 0755)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "opening source") {
		t.Errorf("error = %q, want to contain 'opening source'", err.Error())
	}
}

func TestCopyFileDstNotCreatable(t *testing.T) {
	tmpDir := t.TempDir()

	src := filepath.Join(tmpDir, "src")
	if err := os.WriteFile(src, []byte("content"), 0755); err != nil {
		t.Fatal(err)
	}

	readOnlyDir := filepath.Join(tmpDir, "ro")
	if err := os.MkdirAll(readOnlyDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(readOnlyDir, 0555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(readOnlyDir, 0755) })

	dst := filepath.Join(readOnlyDir, "dst")
	err := copyFile(src, dst, 0755)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "creating destination") {
		t.Errorf("error = %q, want to contain 'creating destination'", err.Error())
	}
}

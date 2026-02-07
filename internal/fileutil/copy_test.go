package fileutil_test

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/lugassawan/rimba/internal/fileutil"
)

func TestCopyDotfiles(t *testing.T) {
	src := t.TempDir()
	dst := t.TempDir()

	// Create some source files
	os.WriteFile(filepath.Join(src, ".env"), []byte("SECRET=123"), 0644)
	os.WriteFile(filepath.Join(src, ".envrc"), []byte("use nix"), 0644)
	// .env.local does NOT exist — should be silently skipped

	files := []string{".env", ".env.local", ".envrc", ".tool-versions"}
	copied, err := fileutil.CopyDotfiles(src, dst, files)
	if err != nil {
		t.Fatalf("CopyDotfiles: %v", err)
	}

	want := []string{".env", ".envrc"}
	if !reflect.DeepEqual(copied, want) {
		t.Errorf("copied = %v, want %v", copied, want)
	}

	// Verify content
	data, err := os.ReadFile(filepath.Join(dst, ".env"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "SECRET=123" {
		t.Errorf(".env content = %q, want %q", data, "SECRET=123")
	}

	// Verify skipped file doesn't exist
	if _, err := os.Stat(filepath.Join(dst, ".env.local")); !os.IsNotExist(err) {
		t.Error(".env.local should not exist in dst")
	}
}

func TestCopyDotfilesPreservesPermissions(t *testing.T) {
	src := t.TempDir()
	dst := t.TempDir()

	os.WriteFile(filepath.Join(src, ".envrc"), []byte("use nix"), 0755)

	copied, err := fileutil.CopyDotfiles(src, dst, []string{".envrc"})
	if err != nil {
		t.Fatalf("CopyDotfiles: %v", err)
	}
	if len(copied) != 1 {
		t.Fatalf("expected 1 copied file, got %d", len(copied))
	}

	info, err := os.Stat(filepath.Join(dst, ".envrc"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0755 {
		t.Errorf("mode = %o, want %o", info.Mode().Perm(), 0755)
	}
}

package fileutil_test

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/lugassawan/rimba/internal/fileutil"
)

const (
	testSecret = "SECRET=123"
	dotEnvrc   = ".envrc"
)

func TestCopyDotfiles(t *testing.T) {
	src := t.TempDir()
	dst := t.TempDir()

	// Create some source files
	os.WriteFile(filepath.Join(src, ".env"), []byte(testSecret), 0644)
	os.WriteFile(filepath.Join(src, dotEnvrc), []byte("use nix"), 0644)
	// .env.local does NOT exist — should be silently skipped

	files := []string{".env", ".env.local", dotEnvrc, ".tool-versions"}
	copied, err := fileutil.CopyDotfiles(src, dst, files)
	if err != nil {
		t.Fatalf("CopyDotfiles: %v", err)
	}

	want := []string{".env", dotEnvrc}
	if !reflect.DeepEqual(copied, want) {
		t.Errorf("copied = %v, want %v", copied, want)
	}

	// Verify content
	data, err := os.ReadFile(filepath.Join(dst, ".env"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != testSecret {
		t.Errorf(".env content = %q, want %q", data, testSecret)
	}

	// Verify skipped file doesn't exist
	if _, err := os.Stat(filepath.Join(dst, ".env.local")); !os.IsNotExist(err) {
		t.Error(".env.local should not exist in dst")
	}
}

func TestCopyDotfilesPreservesPermissions(t *testing.T) {
	src := t.TempDir()
	dst := t.TempDir()

	os.WriteFile(filepath.Join(src, dotEnvrc), []byte("use nix"), 0755)

	copied, err := fileutil.CopyDotfiles(src, dst, []string{dotEnvrc})
	if err != nil {
		t.Fatalf("CopyDotfiles: %v", err)
	}
	if len(copied) != 1 {
		t.Fatalf("expected 1 copied file, got %d", len(copied))
	}

	info, err := os.Stat(filepath.Join(dst, dotEnvrc))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0755 {
		t.Errorf("mode = %o, want %o", info.Mode().Perm(), 0755)
	}
}

func TestCopyDotfilesEmptyList(t *testing.T) {
	src := t.TempDir()
	dst := t.TempDir()

	copied, err := fileutil.CopyDotfiles(src, dst, []string{})
	if err != nil {
		t.Fatalf("CopyDotfiles with empty list: %v", err)
	}
	if copied != nil {
		t.Errorf("expected nil copied list, got %v", copied)
	}
}

func TestCopyDotfilesDstError(t *testing.T) {
	src := t.TempDir()
	dst := t.TempDir()

	// Create a source file that exists
	os.WriteFile(filepath.Join(src, ".env"), []byte(testSecret), 0644)

	// Create a subdirectory at the destination path so OpenFile fails with EISDIR,
	// which is not an os.IsNotExist error and triggers the wrapped error return.
	os.Mkdir(filepath.Join(dst, ".env"), 0755)

	_, err := fileutil.CopyDotfiles(src, dst, []string{".env"})
	if err == nil {
		t.Fatal("expected error when dst path is a directory")
	}
	if !strings.Contains(err.Error(), "copy .env:") {
		t.Errorf("error = %q, want it to contain %q", err, "copy .env:")
	}
}

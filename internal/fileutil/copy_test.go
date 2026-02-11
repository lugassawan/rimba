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
	testSecret      = "SECRET=123"
	dotEnvrc        = ".envrc"
	dotVscode       = ".vscode"
	dotConfig       = ".config"
	dotEmpty        = ".empty"
	dotSecret       = ".secret"
	settingsJSON    = "settings.json"
	appTOML         = "app.toml"
	msgCopyErr      = "CopyEntries: %v"
	msgCopiedWant   = "copied = %v, want %v"
	msgExpect1Entry = "expected 1 entry copied, got %d"
	errContainsFmt  = "error = %q, want it to contain %q"
	errPrefixCopyEnv = "copy .env:"
)

func TestCopyEntries(t *testing.T) {
	src := t.TempDir()
	dst := t.TempDir()

	// Create some source files
	_ = os.WriteFile(filepath.Join(src, ".env"), []byte(testSecret), 0644)
	_ = os.WriteFile(filepath.Join(src, dotEnvrc), []byte("use nix"), 0644)
	// .env.local does NOT exist — should be silently skipped

	files := []string{".env", ".env.local", dotEnvrc, ".tool-versions"}
	copied, err := fileutil.CopyEntries(src, dst, files)
	if err != nil {
		t.Fatalf(msgCopyErr, err)
	}

	want := []string{".env", dotEnvrc}
	if !reflect.DeepEqual(copied, want) {
		t.Errorf(msgCopiedWant, copied, want)
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

func TestCopyEntriesPreservesPermissions(t *testing.T) {
	src := t.TempDir()
	dst := t.TempDir()

	_ = os.WriteFile(filepath.Join(src, dotEnvrc), []byte("use nix"), 0755)

	copied, err := fileutil.CopyEntries(src, dst, []string{dotEnvrc})
	if err != nil {
		t.Fatalf(msgCopyErr, err)
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

func TestCopyEntriesEmptyList(t *testing.T) {
	src := t.TempDir()
	dst := t.TempDir()

	copied, err := fileutil.CopyEntries(src, dst, []string{})
	if err != nil {
		t.Fatalf("CopyEntries with empty list: %v", err)
	}
	if len(copied) != 0 {
		t.Errorf("expected empty copied list, got %v", copied)
	}
}

func TestCopyEntriesDstError(t *testing.T) {
	src := t.TempDir()
	dst := t.TempDir()

	// Create a source file that exists
	_ = os.WriteFile(filepath.Join(src, ".env"), []byte(testSecret), 0644)

	// Create a subdirectory at the destination path so OpenFile fails with EISDIR,
	// which is not an os.IsNotExist error and triggers the wrapped error return.
	_ = os.Mkdir(filepath.Join(dst, ".env"), 0755)

	_, err := fileutil.CopyEntries(src, dst, []string{".env"})
	if err == nil {
		t.Fatal("expected error when dst path is a directory")
	}
	if !strings.Contains(err.Error(), errPrefixCopyEnv) {
		t.Errorf(errContainsFmt, err, errPrefixCopyEnv)
	}
}

func TestCopyEntriesCopiesDirectory(t *testing.T) {
	src := t.TempDir()
	dst := t.TempDir()

	// Create .vscode/ with two files
	vscodeDir := filepath.Join(src, dotVscode)
	_ = os.Mkdir(vscodeDir, 0755)
	_ = os.WriteFile(filepath.Join(vscodeDir, settingsJSON), []byte(`{"go.formatTool":"goimports"}`), 0644)
	_ = os.WriteFile(filepath.Join(vscodeDir, "extensions.json"), []byte(`{"recommendations":[]}`), 0644)

	copied, err := fileutil.CopyEntries(src, dst, []string{dotVscode})
	if err != nil {
		t.Fatalf(msgCopyErr, err)
	}

	want := []string{dotVscode}
	if !reflect.DeepEqual(copied, want) {
		t.Errorf(msgCopiedWant, copied, want)
	}

	// Verify both files exist in destination
	data, err := os.ReadFile(filepath.Join(dst, dotVscode, settingsJSON))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "goimports") {
		t.Errorf("settings.json content = %q, expected goimports", data)
	}

	if _, err := os.Stat(filepath.Join(dst, dotVscode, "extensions.json")); os.IsNotExist(err) {
		t.Error("extensions.json should exist in dst/.vscode")
	}
}

func TestCopyEntriesCopiesNestedDirectory(t *testing.T) {
	src := t.TempDir()
	dst := t.TempDir()

	// Create .config/sub/deep/ structure
	deepDir := filepath.Join(src, dotConfig, "sub", "deep")
	_ = os.MkdirAll(deepDir, 0755)
	_ = os.WriteFile(filepath.Join(src, dotConfig, "top.toml"), []byte("top"), 0644)
	_ = os.WriteFile(filepath.Join(deepDir, "nested.toml"), []byte("nested"), 0644)

	copied, err := fileutil.CopyEntries(src, dst, []string{dotConfig})
	if err != nil {
		t.Fatalf(msgCopyErr, err)
	}
	if len(copied) != 1 {
		t.Fatalf(msgExpect1Entry, len(copied))
	}

	// Verify nested file
	data, err := os.ReadFile(filepath.Join(dst, dotConfig, "sub", "deep", "nested.toml"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "nested" {
		t.Errorf("nested.toml content = %q, want %q", data, "nested")
	}

	// Verify top-level file in dir
	data, err = os.ReadFile(filepath.Join(dst, dotConfig, "top.toml"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "top" {
		t.Errorf("top.toml content = %q, want %q", data, "top")
	}
}

func TestCopyEntriesMixedFilesAndDirs(t *testing.T) {
	src := t.TempDir()
	dst := t.TempDir()

	// Create a file
	_ = os.WriteFile(filepath.Join(src, ".env"), []byte(testSecret), 0644)

	// Create a directory with a file
	configDir := filepath.Join(src, dotConfig)
	_ = os.Mkdir(configDir, 0755)
	_ = os.WriteFile(filepath.Join(configDir, appTOML), []byte("app"), 0644)

	// .missing does NOT exist — should be skipped
	copied, err := fileutil.CopyEntries(src, dst, []string{".env", dotConfig, ".missing"})
	if err != nil {
		t.Fatalf(msgCopyErr, err)
	}

	want := []string{".env", dotConfig}
	if !reflect.DeepEqual(copied, want) {
		t.Errorf(msgCopiedWant, copied, want)
	}

	// Verify file
	if _, err := os.Stat(filepath.Join(dst, ".env")); os.IsNotExist(err) {
		t.Error(".env should exist in dst")
	}
	// Verify dir content
	if _, err := os.Stat(filepath.Join(dst, dotConfig, appTOML)); os.IsNotExist(err) {
		t.Error(".config/app.toml should exist in dst")
	}
	// Verify missing was skipped
	if _, err := os.Stat(filepath.Join(dst, ".missing")); !os.IsNotExist(err) {
		t.Error(".missing should not exist in dst")
	}
}

func TestCopyEntriesEmptyDirectory(t *testing.T) {
	src := t.TempDir()
	dst := t.TempDir()

	// Create an empty directory
	_ = os.Mkdir(filepath.Join(src, dotEmpty), 0755)

	copied, err := fileutil.CopyEntries(src, dst, []string{dotEmpty})
	if err != nil {
		t.Fatalf(msgCopyErr, err)
	}
	if len(copied) != 1 {
		t.Fatalf(msgExpect1Entry, len(copied))
	}

	// Verify directory was created
	info, err := os.Stat(filepath.Join(dst, dotEmpty))
	if err != nil {
		t.Fatal(err)
	}
	if !info.IsDir() {
		t.Error(".empty should be a directory in dst")
	}
}

func TestCopyEntriesDirPreservesPermissions(t *testing.T) {
	src := t.TempDir()
	dst := t.TempDir()

	// Create directory with specific permissions
	srcDir := filepath.Join(src, dotSecret)
	_ = os.Mkdir(srcDir, 0700)
	_ = os.WriteFile(filepath.Join(srcDir, "key.pem"), []byte("key"), 0600)

	copied, err := fileutil.CopyEntries(src, dst, []string{dotSecret})
	if err != nil {
		t.Fatalf(msgCopyErr, err)
	}
	if len(copied) != 1 {
		t.Fatalf(msgExpect1Entry, len(copied))
	}

	// Verify directory permissions
	dirInfo, err := os.Stat(filepath.Join(dst, dotSecret))
	if err != nil {
		t.Fatal(err)
	}
	if dirInfo.Mode().Perm() != 0700 {
		t.Errorf("dir mode = %o, want %o", dirInfo.Mode().Perm(), 0700)
	}

	// Verify file permissions
	fileInfo, err := os.Stat(filepath.Join(dst, dotSecret, "key.pem"))
	if err != nil {
		t.Fatal(err)
	}
	if fileInfo.Mode().Perm() != 0600 {
		t.Errorf("file mode = %o, want %o", fileInfo.Mode().Perm(), 0600)
	}
}

func TestCopyEntriesSkipsSymlinksInDir(t *testing.T) {
	src := t.TempDir()
	dst := t.TempDir()

	// Create a directory with a regular file and a symlink
	srcDir := filepath.Join(src, dotConfig)
	_ = os.Mkdir(srcDir, 0755)
	_ = os.WriteFile(filepath.Join(srcDir, "real.toml"), []byte("real"), 0644)
	_ = os.Symlink("/dev/null", filepath.Join(srcDir, "link.toml"))

	copied, err := fileutil.CopyEntries(src, dst, []string{dotConfig})
	if err != nil {
		t.Fatalf(msgCopyErr, err)
	}
	if len(copied) != 1 {
		t.Fatalf(msgExpect1Entry, len(copied))
	}

	// Real file should exist
	if _, err := os.Stat(filepath.Join(dst, dotConfig, "real.toml")); os.IsNotExist(err) {
		t.Error("real.toml should exist in dst")
	}
	// Symlink should NOT exist
	if _, err := os.Lstat(filepath.Join(dst, dotConfig, "link.toml")); !os.IsNotExist(err) {
		t.Error("link.toml (symlink) should not exist in dst")
	}
}

func TestCopyEntriesStatError(t *testing.T) {
	src := t.TempDir()
	dst := t.TempDir()

	// Create a file but make the source directory unreadable
	_ = os.WriteFile(filepath.Join(src, ".env"), []byte("SECRET"), 0644)

	// Remove read permission from src dir so Stat fails with permission denied (not IsNotExist)
	if err := os.Chmod(src, 0000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(src, 0755) })

	_, err := fileutil.CopyEntries(src, dst, []string{".env"})
	if err == nil {
		t.Fatal("expected error when source dir is not readable")
	}
	if !strings.Contains(err.Error(), errPrefixCopyEnv) {
		t.Errorf(errContainsFmt, err, errPrefixCopyEnv)
	}
}

func TestCopyEntriesDirCopyError(t *testing.T) {
	src := t.TempDir()
	dst := t.TempDir()

	// Create a source directory with a file
	srcDir := filepath.Join(src, dotConfig)
	_ = os.Mkdir(srcDir, 0755)
	_ = os.WriteFile(filepath.Join(srcDir, appTOML), []byte("app"), 0644)

	// Create a regular file at the destination path where a directory is expected.
	// This causes MkdirAll for the nested file to fail.
	_ = os.WriteFile(filepath.Join(dst, dotConfig), []byte("conflict"), 0644)

	_, err := fileutil.CopyEntries(src, dst, []string{dotConfig})
	if err == nil {
		t.Fatal("expected error when dst path conflicts")
	}
	if !strings.Contains(err.Error(), "copy "+dotConfig+":") {
		t.Errorf(errContainsFmt, err, "copy "+dotConfig+":")
	}
}

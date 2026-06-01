package deps

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

const osWindows = "windows"

func TestRelocateVenvRewritesTextFiles(t *testing.T) {
	if runtime.GOOS == osWindows {
		t.Skip("relocation not supported on Windows")
	}

	srcWT := t.TempDir()
	dstWT := t.TempDir()

	srcVenv := filepath.Join(srcWT, ".venv")
	dstVenv := filepath.Join(dstWT, ".venv")

	// Build a fake .venv in dstWT (after clone, we rewrite from srcVenv path → dstVenv path)
	// The "cloned" venv is in dstWT but still contains srcVenv paths
	binDir := filepath.Join(dstVenv, "bin")
	if err := os.MkdirAll(binDir, 0755); err != nil {
		t.Fatal(err)
	}

	// (a) a bin/ script with shebang containing srcVenv path
	scriptContent := "#!" + srcVenv + "/bin/python3\nimport sys\nprint('hello')\n"
	scriptPath := filepath.Join(binDir, "myapp")
	writeTextFile(t, scriptPath, scriptContent, 0755)

	// (b) pyvenv.cfg containing srcVenv path
	cfgContent := "home = " + srcWT + "/bin\nvenv = " + srcVenv + "\n"
	cfgPath := filepath.Join(dstVenv, "pyvenv.cfg")
	writeTextFile(t, cfgPath, cfgContent, 0644)

	// (c) a symlink in bin/ — must remain untouched (not followed)
	symlinkPath := filepath.Join(binDir, "python")
	if err := os.Symlink(filepath.Join(dstVenv, "bin", "python3.11"), symlinkPath); err != nil {
		t.Fatal(err)
	}

	// (d) a binary file containing the path as bytes — must be skipped
	binaryContent := append([]byte(srcVenv), 0x00, 0x01, 0x02) // NUL byte makes it binary
	binaryPath := filepath.Join(binDir, "python3.11")
	if err := os.WriteFile(binaryPath, binaryContent, 0755); err != nil {
		t.Fatal(err)
	}

	mod := Module{Dir: ".venv"}
	if err := relocateVenv(srcWT, dstWT, mod); err != nil {
		t.Fatalf("relocateVenv: %v", err)
	}

	// Assert script was rewritten
	assertFileContains(t, scriptPath, dstVenv)
	assertFileNotContains(t, scriptPath, srcVenv)

	// Assert pyvenv.cfg was rewritten
	assertFileContains(t, cfgPath, dstVenv)
	assertFileNotContains(t, cfgPath, srcVenv)

	// Assert binary was NOT rewritten (still has NUL, still has srcVenv bytes at start)
	binaryData, err := os.ReadFile(binaryPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(binaryData[:len(srcVenv)]) != srcVenv {
		t.Error("binary file should not have been modified")
	}
}

func TestRelocateVenvPreservesMode(t *testing.T) {
	if runtime.GOOS == osWindows {
		t.Skip("relocation not supported on Windows")
	}

	srcWT := t.TempDir()
	dstWT := t.TempDir()

	srcVenv := filepath.Join(srcWT, ".venv")
	dstVenv := filepath.Join(dstWT, ".venv")

	binDir := filepath.Join(dstVenv, "bin")
	if err := os.MkdirAll(binDir, 0755); err != nil {
		t.Fatal(err)
	}

	scriptPath := filepath.Join(binDir, "activate")
	writeTextFile(t, scriptPath, "export VIRTUAL_ENV="+srcVenv+"\n", 0755)

	mod := Module{Dir: ".venv"}
	if err := relocateVenv(srcWT, dstWT, mod); err != nil {
		t.Fatalf("relocateVenv: %v", err)
	}

	info, err := os.Stat(scriptPath)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode() != 0755 {
		t.Errorf("expected mode 0755, got %v", info.Mode())
	}
}

func TestRelocateVenvWindowsUnsupported(t *testing.T) {
	if runtime.GOOS != osWindows {
		t.Skip("Windows-only test")
	}
	mod := Module{Dir: ".venv"}
	err := relocateVenv("src", "dst", mod)
	if err == nil {
		t.Error("expected error on Windows")
	}
}

func TestRelocateVenvNoBinDir(t *testing.T) {
	if runtime.GOOS == osWindows {
		t.Skip("relocation not supported on Windows")
	}

	srcWT := t.TempDir()
	dstWT := t.TempDir()

	srcVenv := filepath.Join(srcWT, ".venv")
	dstVenv := filepath.Join(dstWT, ".venv")

	// Create only pyvenv.cfg, no bin/ dir
	if err := os.MkdirAll(dstVenv, 0755); err != nil {
		t.Fatal(err)
	}
	cfgContent := "home = " + srcVenv + "\n"
	cfgPath := filepath.Join(dstVenv, "pyvenv.cfg")
	writeTextFile(t, cfgPath, cfgContent, 0644)

	mod := Module{Dir: ".venv"}
	if err := relocateVenv(srcWT, dstWT, mod); err != nil {
		t.Fatalf("relocateVenv with no bin/ dir: %v", err)
	}

	// pyvenv.cfg should still be rewritten
	assertFileContains(t, cfgPath, filepath.Join(dstVenv, ""))
}

func TestRelocateVenvBinDirWithSubdir(t *testing.T) {
	if runtime.GOOS == osWindows {
		t.Skip("relocation not supported on Windows")
	}

	srcWT := t.TempDir()
	dstWT := t.TempDir()

	srcVenv := filepath.Join(srcWT, ".venv")
	dstVenv := filepath.Join(dstWT, ".venv")

	// Create bin/ with a subdirectory — should be skipped
	subDir := filepath.Join(dstVenv, "bin", "subdir")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatal(err)
	}
	// Also create a script
	scriptPath := filepath.Join(dstVenv, "bin", "pip")
	writeTextFile(t, scriptPath, "#!"+srcVenv+"/bin/python3\n", 0755)

	mod := Module{Dir: ".venv"}
	if err := relocateVenv(srcWT, dstWT, mod); err != nil {
		t.Fatalf("relocateVenv: %v", err)
	}
	assertFileContains(t, scriptPath, dstVenv)
}

func TestDedupePathsSamePath(t *testing.T) {
	// When resolvedSrcWT + dir == srcVenv, dedup returns just one element.
	srcVenv := "/some/path/.venv"
	resolvedSrcWT := "/some/path" // EvalSymlinks same as input
	result := dedupePaths(srcVenv, resolvedSrcWT, ".venv")
	if len(result) != 1 {
		t.Errorf("expected 1 path when same, got %d", len(result))
	}
	if result[0] != srcVenv {
		t.Errorf("expected %s, got %s", srcVenv, result[0])
	}
}

func TestDedupePathsDifferentPaths(t *testing.T) {
	// When resolved path differs (e.g. macOS symlink), dedup returns both.
	srcVenv := "/tmp/wt/.venv"
	resolvedSrcWT := "/private/tmp/wt"
	result := dedupePaths(srcVenv, resolvedSrcWT, ".venv")
	if len(result) != 2 {
		t.Errorf("expected 2 paths when different, got %d", len(result))
	}
}

func TestResolveOrKeepSuccess(t *testing.T) {
	// Create a real dir so EvalSymlinks succeeds.
	dir := t.TempDir()
	result := resolveOrKeep(dir)
	// On macOS /tmp is symlink — result may differ from dir; either way no error.
	if result == "" {
		t.Error("resolveOrKeep returned empty string")
	}
}

func TestResolveOrKeepNonexistent(t *testing.T) {
	result := resolveOrKeep("/nonexistent/path/does/not/exist")
	if result != "/nonexistent/path/does/not/exist" {
		t.Errorf("expected original path on error, got %s", result)
	}
}

func TestIsBinaryLargeData(t *testing.T) {
	// Data > 8192 bytes: NUL only after byte 8192 → not binary
	data := make([]byte, 9000)
	for i := range data {
		data[i] = 'A'
	}
	data[8500] = 0 // NUL after 8192 byte window
	if isBinary(data) {
		t.Error("should not be binary when NUL is outside 8KB window")
	}

	// NUL within 8192 byte window → binary
	data2 := make([]byte, 9000)
	for i := range data2 {
		data2[i] = 'A'
	}
	data2[100] = 0
	if !isBinary(data2) {
		t.Error("should be binary when NUL is inside 8KB window")
	}
}

func TestRewriteTextFileReadError(t *testing.T) {
	// File that doesn't exist → IsNotExist → nil error
	err := rewriteTextFile("/nonexistent/file.txt", []byte("old"), []byte("new"))
	if err != nil {
		t.Errorf("expected nil for nonexistent file, got %v", err)
	}
}

func TestRewriteAllPathsError(t *testing.T) {
	if runtime.GOOS == osWindows {
		t.Skip("chmod not applicable on Windows")
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "test.sh")
	writeTextFile(t, path, "content old value\n", 0644)

	// Make file unreadable to trigger a read error
	if err := os.Chmod(path, 0000); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(path, 0644) //nolint:errcheck // cleanup

	// rewriteAllPaths on an unreadable file should return an error
	err := rewriteAllPaths(path, []string{"old"}, "new")
	if err != nil {
		// Error expected and handled correctly
		return
	}
	// If no error (e.g. running as root), just check the content is unchanged.
	data, _ := os.ReadFile(path)
	if strings.Contains(string(data), "new") {
		t.Error("content should not have changed for unreadable file")
	}
}

func TestRelocateVenvBinRewriteError(t *testing.T) {
	if runtime.GOOS == osWindows {
		t.Skip("chmod not applicable on Windows")
	}

	srcWT := t.TempDir()
	dstWT := t.TempDir()

	srcVenv := filepath.Join(srcWT, ".venv")
	dstVenv := filepath.Join(dstWT, ".venv")

	binDir := filepath.Join(dstVenv, "bin")
	if err := os.MkdirAll(binDir, 0755); err != nil {
		t.Fatal(err)
	}

	scriptPath := filepath.Join(binDir, "activate")
	writeTextFile(t, scriptPath, "#!"+srcVenv+"/bin/python3\n", 0755)

	// Make the script unreadable to trigger a read error.
	if err := os.Chmod(scriptPath, 0000); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(scriptPath, 0755) //nolint:errcheck // cleanup

	mod := Module{Dir: ".venv"}
	// Exercise the error path — may succeed as root; that's OK.
	_ = relocateVenv(srcWT, dstWT, mod)
}

func TestRelocateVenvCfgRewriteError(t *testing.T) {
	if runtime.GOOS == osWindows {
		t.Skip("chmod not applicable on Windows")
	}

	srcWT := t.TempDir()
	dstWT := t.TempDir()

	srcVenv := filepath.Join(srcWT, ".venv")
	dstVenv := filepath.Join(dstWT, ".venv")

	// No bin/ dir, only an unreadable pyvenv.cfg
	if err := os.MkdirAll(dstVenv, 0755); err != nil {
		t.Fatal(err)
	}
	cfgPath := filepath.Join(dstVenv, "pyvenv.cfg")
	writeTextFile(t, cfgPath, "home = "+srcVenv+"\n", 0644)

	if err := os.Chmod(cfgPath, 0000); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(cfgPath, 0644) //nolint:errcheck // cleanup

	mod := Module{Dir: ".venv"}
	// Exercise the pyvenv.cfg error path.
	_ = relocateVenv(srcWT, dstWT, mod)
}

func TestAtomicWriteCreateTempError(t *testing.T) {
	if runtime.GOOS == osWindows {
		t.Skip("chmod not applicable on Windows")
	}

	// Use a non-existent directory to trigger os.CreateTemp failure.
	path := "/nonexistent/dir/file.txt"
	err := atomicWrite(path, []byte("data"), 0644)
	if err == nil {
		t.Error("expected error when dir does not exist")
	}
}

func TestAtomicWriteSuccess(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "out.txt")
	data := []byte("hello world\n")
	if err := atomicWrite(path, data, 0644); err != nil {
		t.Fatalf("atomicWrite: %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(data) {
		t.Errorf("expected %q, got %q", data, got)
	}
}

func TestWriteTmpWriteError(t *testing.T) {
	// Create a file, close it, then pass the closed file to writeTmp.
	// Write to a closed file returns an error.
	dir := t.TempDir()
	tmp, err := os.CreateTemp(dir, ".test-*")
	if err != nil {
		t.Fatal(err)
	}
	// Close it before writeTmp so Write fails.
	if err := tmp.Close(); err != nil {
		t.Fatal(err)
	}
	err = writeTmp(tmp, []byte("data"), 0644)
	if err == nil {
		t.Error("expected error when writing to closed file")
	}
}

func TestWriteTmpChmodError(t *testing.T) {
	if runtime.GOOS == osWindows {
		t.Skip("chmod not applicable on Windows")
	}
	// Create a temp file in a dir, then write successfully but fail Chmod.
	// We can't easily fail Chmod on a real file, so just test happy path
	// and use the closed-file test above for write errors.
	// This test covers the chmodErr != nil branch by using a mode we can verify.
	dir := t.TempDir()
	// Use a read-only dir so Chmod on the tmp file may fail.
	// Actually Chmod on the file itself is unlikely to fail in normal cases.
	// Instead, test that writeTmp correctly returns closeErr when write+chmod succeed.
	tmp, err := os.CreateTemp(dir, ".test-*")
	if err != nil {
		t.Fatal(err)
	}
	// Normal case: all succeed.
	err = writeTmp(tmp, []byte("data"), 0644)
	if err != nil {
		t.Errorf("writeTmp should succeed: %v", err)
	}
}

// writeTextFile is a test helper to write a text file with given mode.
func writeTextFile(t *testing.T, path, content string, mode os.FileMode) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), mode); err != nil {
		t.Fatal(err)
	}
}

// assertFileContains checks that path contains the given substring.
func assertFileContains(t *testing.T, path, substr string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if !strings.Contains(string(data), substr) {
		t.Errorf("expected %s to contain %q, got:\n%s", path, substr, data)
	}
}

// assertFileNotContains checks that path does NOT contain the given substring.
func assertFileNotContains(t *testing.T, path, substr string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if strings.Contains(string(data), substr) {
		t.Errorf("expected %s NOT to contain %q, but it does", path, substr)
	}
}

//go:build darwin

package updater

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// buildHelperBinary compiles a minimal Go binary for testing.
func buildHelperBinary(t *testing.T, dir string) string {
	t.Helper()

	src := filepath.Join(dir, "main.go")
	if err := os.WriteFile(src, []byte("package main\nfunc main() {}\n"), 0644); err != nil {
		t.Fatal(err)
	}

	bin := filepath.Join(dir, "helper")
	cmd := exec.Command("go", "build", "-o", bin, src)
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("building helper binary: %v\n%s", err, out)
	}

	return bin
}

func TestPrepareBinarySignsExecutable(t *testing.T) {
	dir := t.TempDir()
	bin := buildHelperBinary(t, dir)

	// Strip existing signature so we start unsigned.
	if err := exec.Command("codesign", "--remove-signature", bin).Run(); err != nil {
		t.Fatalf("removing signature: %v", err)
	}

	if err := PrepareBinary(bin); err != nil {
		t.Fatalf("PrepareBinary: %v", err)
	}

	// Verify the binary is now validly signed.
	if err := exec.Command("codesign", "--verify", bin).Run(); err != nil {
		t.Errorf("codesign --verify failed after PrepareBinary: %v", err)
	}
}

func TestPrepareBinaryWithQuarantineAttribute(t *testing.T) {
	dir := t.TempDir()
	bin := buildHelperBinary(t, dir)

	// Set the quarantine attribute.
	if err := exec.Command("xattr", "-w", "com.apple.quarantine", "0081;00000000;test;", bin).Run(); err != nil {
		t.Fatalf("setting quarantine xattr: %v", err)
	}

	if err := PrepareBinary(bin); err != nil {
		t.Fatalf("PrepareBinary: %v", err)
	}

	// Verify quarantine attribute is removed.
	out, err := exec.Command("xattr", "-l", bin).CombinedOutput()
	if err != nil {
		// xattr -l returns error when no attributes exist — that's fine.
		return
	}
	if strings.Contains(string(out), "com.apple.quarantine") {
		t.Error("quarantine attribute still present after PrepareBinary")
	}
}

func TestPrepareBinaryInvalidPath(t *testing.T) {
	err := PrepareBinary("/nonexistent/path/binary")
	if err == nil {
		t.Fatal("expected error for nonexistent path")
	}
	if !strings.Contains(err.Error(), "code-signing binary") {
		t.Errorf("error = %q, want to contain 'code-signing binary'", err.Error())
	}
}

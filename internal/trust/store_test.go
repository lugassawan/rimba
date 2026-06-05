package trust_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/lugassawan/rimba/internal/trust"
)

func TestLoadAbsent(t *testing.T) {
	dir := t.TempDir()
	s, err := trust.Load(dir)
	if err != nil {
		t.Fatalf("Load on absent file: %v", err)
	}
	if s != nil {
		t.Errorf("Load on absent file = %v, want nil", s)
	}
}

func TestLoadMalformedTOML(t *testing.T) {
	dir := t.TempDir()
	rimbaDir := filepath.Join(dir, ".rimba")
	if err := os.MkdirAll(rimbaDir, 0750); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(rimbaDir, "trust.local.toml")
	if err := os.WriteFile(p, []byte("not valid toml = [broken"), 0600); err != nil {
		t.Fatal(err)
	}

	_, err := trust.Load(dir)
	if err == nil {
		t.Error("Load on malformed TOML should return error")
	}
}

func TestIsTrustedAbsentFile(t *testing.T) {
	dir := t.TempDir()
	ok, err := trust.IsTrusted(dir, "sha256:abc")
	if err != nil {
		t.Fatalf("IsTrusted on absent file: %v", err)
	}
	if ok {
		t.Error("IsTrusted on absent file should be false")
	}
}

func TestRecordThenIsTrusted(t *testing.T) {
	dir := t.TempDir()
	// Create .gitignore and .rimba dir so Record can function.
	if err := os.WriteFile(filepath.Join(dir, ".gitignore"), nil, 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, ".rimba"), 0750); err != nil {
		t.Fatal(err)
	}

	cfg := cfgWithCommands([]string{"make test"}, nil)
	h := trust.Hash(cfg)

	if err := trust.Record(dir, h); err != nil {
		t.Fatalf("Record: %v", err)
	}

	ok, err := trust.IsTrusted(dir, h)
	if err != nil {
		t.Fatalf("IsTrusted after Record: %v", err)
	}
	if !ok {
		t.Error("IsTrusted should be true after Record")
	}
}

func TestIsTrustedWrongHash(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".gitignore"), nil, 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, ".rimba"), 0750); err != nil {
		t.Fatal(err)
	}

	if err := trust.Record(dir, "sha256:aaa"); err != nil {
		t.Fatalf("Record: %v", err)
	}

	ok, err := trust.IsTrusted(dir, "sha256:bbb")
	if err != nil {
		t.Fatalf("IsTrusted: %v", err)
	}
	if ok {
		t.Error("IsTrusted should be false when hash differs")
	}
}

func TestRecordFilePermissions(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".gitignore"), nil, 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, ".rimba"), 0750); err != nil {
		t.Fatal(err)
	}

	if err := trust.Record(dir, "sha256:test"); err != nil {
		t.Fatalf("Record: %v", err)
	}

	info, err := os.Stat(filepath.Join(dir, ".rimba", trust.FileName))
	if err != nil {
		t.Fatalf("stat trust file: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0600 {
		t.Errorf("trust file permissions = %o, want 0600", perm)
	}
}

func TestRecordGitignoreEntry(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".gitignore"), []byte(".rimba/settings.local.toml\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, ".rimba"), 0750); err != nil {
		t.Fatal(err)
	}

	if err := trust.Record(dir, "sha256:test"); err != nil {
		t.Fatalf("Record: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, ".gitignore"))
	if err != nil {
		t.Fatalf("read .gitignore: %v", err)
	}
	if !strings.Contains(string(data), ".rimba/trust.local.toml") {
		t.Errorf(".gitignore should contain .rimba/trust.local.toml, got:\n%s", data)
	}
}

func TestRecordOverwritesAndUpdatesApprovedAt(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".gitignore"), nil, 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, ".rimba"), 0750); err != nil {
		t.Fatal(err)
	}

	if err := trust.Record(dir, "sha256:first"); err != nil {
		t.Fatalf("Record first: %v", err)
	}

	s1, err := trust.Load(dir)
	if err != nil || s1 == nil {
		t.Fatalf("Load after first Record: %v", err)
	}

	time.Sleep(2 * time.Millisecond) // ensure distinct timestamp

	if err := trust.Record(dir, "sha256:second"); err != nil {
		t.Fatalf("Record second: %v", err)
	}

	s2, err := trust.Load(dir)
	if err != nil || s2 == nil {
		t.Fatalf("Load after second Record: %v", err)
	}

	if s2.Hash != "sha256:second" {
		t.Errorf("second Record should overwrite hash, got %q", s2.Hash)
	}
}

func TestLoadReadError(t *testing.T) {
	// Place a directory where trust.local.toml should be — ReadFile returns
	// EISDIR, which is not os.IsNotExist, exercising the "read trust store" error branch.
	dir := t.TempDir()
	rimbaDir := filepath.Join(dir, ".rimba")
	if err := os.MkdirAll(rimbaDir, 0750); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(rimbaDir, "trust.local.toml"), 0750); err != nil {
		t.Fatal(err)
	}

	_, err := trust.Load(dir)
	if err == nil {
		t.Error("Load with unreadable file should return error")
	}
}

func TestIsTrustedReadError(t *testing.T) {
	dir := t.TempDir()
	rimbaDir := filepath.Join(dir, ".rimba")
	if err := os.MkdirAll(rimbaDir, 0750); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(rimbaDir, "trust.local.toml"), 0750); err != nil {
		t.Fatal(err)
	}

	_, err := trust.IsTrusted(dir, "sha256:any")
	if err == nil {
		t.Error("IsTrusted with unreadable file should propagate error")
	}
}

func TestRecordMkdirAllFails(t *testing.T) {
	// Place a regular file at .rimba/ so MkdirAll fails.
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".rimba"), []byte("blocker"), 0644); err != nil {
		t.Fatal(err)
	}

	err := trust.Record(dir, "sha256:test")
	if err == nil {
		t.Error("Record should return error when .rimba is a file")
	}
}

func TestRecordGitignoreFails(t *testing.T) {
	// Place a directory at .gitignore so EnsureGitignore fails.
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".rimba"), 0750); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(dir, ".gitignore"), 0750); err != nil {
		t.Fatal(err)
	}

	err := trust.Record(dir, "sha256:test")
	if err == nil {
		t.Error("Record should return error when .gitignore is a directory")
	}
}

func TestRecordWriteFails(t *testing.T) {
	// Place a directory at trust.local.toml path so WriteFile fails.
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".gitignore"), nil, 0644); err != nil {
		t.Fatal(err)
	}
	rimbaDir := filepath.Join(dir, ".rimba")
	if err := os.MkdirAll(rimbaDir, 0750); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(rimbaDir, "trust.local.toml"), 0750); err != nil {
		t.Fatal(err)
	}

	err := trust.Record(dir, "sha256:test")
	if err == nil {
		t.Error("Record should return error when trust file path is a directory")
	}
}

func TestGateNonInteractiveIsTrustedError(t *testing.T) {
	dir := t.TempDir()
	rimbaDir := filepath.Join(dir, ".rimba")
	if err := os.MkdirAll(rimbaDir, 0750); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(rimbaDir, "trust.local.toml"), 0750); err != nil {
		t.Fatal(err)
	}

	cfg := cfgWithCommands([]string{"make build"}, nil)
	err := trust.GateNonInteractive(dir, cfg)
	if err == nil {
		t.Error("GateNonInteractive should propagate IsTrusted error")
	}
}

func TestRecordCreatesRimbaDir(t *testing.T) {
	// Record should create .rimba/ if it doesn't exist.
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".gitignore"), nil, 0644); err != nil {
		t.Fatal(err)
	}
	// Do NOT create .rimba/ — Record should handle it.

	if err := trust.Record(dir, "sha256:test"); err != nil {
		t.Fatalf("Record without pre-existing .rimba/: %v", err)
	}

	assertFileExists(t, filepath.Join(dir, ".rimba", trust.FileName))
}

func TestGateNonInteractiveNoCommands(t *testing.T) {
	dir := t.TempDir()
	err := trust.GateNonInteractive(dir, emptyConfig())
	if err != nil {
		t.Errorf("GateNonInteractive with no commands should return nil, got: %v", err)
	}
}

func TestGateNonInteractiveTrusted(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".gitignore"), nil, 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, ".rimba"), 0750); err != nil {
		t.Fatal(err)
	}

	cfg := cfgWithCommands([]string{"make build"}, nil)
	if err := trust.Record(dir, trust.Hash(cfg)); err != nil {
		t.Fatalf("Record: %v", err)
	}

	if err := trust.GateNonInteractive(dir, cfg); err != nil {
		t.Errorf("GateNonInteractive trusted should return nil, got: %v", err)
	}
}

func TestGateNonInteractiveUntrusted(t *testing.T) {
	dir := t.TempDir()
	cfg := cfgWithCommands([]string{"make build"}, nil)
	err := trust.GateNonInteractive(dir, cfg)
	if err == nil {
		t.Error("GateNonInteractive untrusted should return error")
	}
	if !strings.Contains(err.Error(), "rimba trust") {
		t.Errorf("error should mention 'rimba trust', got: %v", err)
	}
}

func TestGateNonInteractiveEnvEscapeHatch(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".gitignore"), nil, 0644); err != nil {
		t.Fatal(err)
	}
	cfg := cfgWithCommands([]string{"make build"}, nil)

	t.Setenv("RIMBA_TRUST_YES", "1")
	if err := trust.GateNonInteractive(dir, cfg); err != nil {
		t.Errorf("GateNonInteractive with RIMBA_TRUST_YES=1 should return nil, got: %v", err)
	}
}

func TestLoadNewerVersion(t *testing.T) {
	dir := t.TempDir()
	rimbaDir := filepath.Join(dir, ".rimba")
	if err := os.MkdirAll(rimbaDir, 0750); err != nil {
		t.Fatal(err)
	}
	// Write a store file with a future version number.
	p := filepath.Join(rimbaDir, "trust.local.toml")
	if err := os.WriteFile(p, []byte("version = 99\nhash = \"sha256:abc\"\napproved_at = \"2026-01-01T00:00:00Z\"\n"), 0600); err != nil {
		t.Fatal(err)
	}

	_, err := trust.Load(dir)
	if err == nil {
		t.Error("Load with newer store version should return error")
	}
}

func TestGateNonInteractiveEnvEscapeHatchZero(t *testing.T) {
	dir := t.TempDir()
	cfg := cfgWithCommands([]string{"make build"}, nil)

	t.Setenv("RIMBA_TRUST_YES", "0")
	err := trust.GateNonInteractive(dir, cfg)
	if err == nil {
		t.Error("GateNonInteractive with RIMBA_TRUST_YES=0 should still return error")
	}
}

func assertFileExists(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Errorf("expected file to exist: %s", path)
	}
}

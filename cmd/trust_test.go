package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lugassawan/rimba/internal/config"
	"github.com/lugassawan/rimba/internal/trust"
	"github.com/spf13/cobra"
)

// newTrustCmd builds a testable trustCmd with its flags registered.
func newTrustCmd(cfg *config.Config, repoRoot string) (*cobra.Command, *bytes.Buffer) {
	buf := &bytes.Buffer{}
	c := buildTrustCmd(cfg, repoRoot)
	c.SetOut(buf)
	c.SetErr(buf)
	return c, buf
}

func TestTrustCmdApprove(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".gitignore"), []byte(".rimba/settings.local.toml\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, ".rimba"), 0750); err != nil {
		t.Fatal(err)
	}

	cfg := &config.Config{PostCreate: []string{"make install"}}
	cmd, buf := newTrustCmd(cfg, dir)
	cmd.SetIn(strings.NewReader("y\n"))

	if err := cmd.Execute(); err != nil {
		t.Fatalf("trust approve: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "Approved") {
		t.Errorf("expected 'Approved' in output, got: %s", out)
	}

	ok, _ := trust.IsTrusted(dir, trust.Hash(cfg))
	if !ok {
		t.Error("trust should be recorded after approve")
	}
}

func TestTrustCmdAlreadyTrusted(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".gitignore"), nil, 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, ".rimba"), 0750); err != nil {
		t.Fatal(err)
	}

	cfg := &config.Config{PostCreate: []string{"npm ci"}}
	if err := trust.Record(dir, trust.Hash(cfg)); err != nil {
		t.Fatal(err)
	}

	cmd, buf := newTrustCmd(cfg, dir)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("trust already trusted: %v", err)
	}

	if !strings.Contains(strings.ToLower(buf.String()), "already trusted") {
		t.Errorf("expected 'already trusted' in output, got: %s", buf.String())
	}
}

func TestTrustCmdNoCommands(t *testing.T) {
	dir := t.TempDir()
	cmd, buf := newTrustCmd(&config.Config{}, dir)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("trust no commands: %v", err)
	}
	if !strings.Contains(strings.ToLower(buf.String()), "no shell commands") {
		t.Errorf("expected 'no shell commands' in output, got: %s", buf.String())
	}
}

func TestTrustCmdShowNoCommands(t *testing.T) {
	dir := t.TempDir()
	cmd, buf := newTrustCmd(&config.Config{}, dir)
	cmd.SetArgs([]string{"--show"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("trust --show no commands: %v", err)
	}
	if !strings.Contains(strings.ToLower(buf.String()), "no shell commands") {
		t.Errorf("expected 'no shell commands' in output, got: %s", buf.String())
	}
}

func TestTrustCmdShow(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".gitignore"), nil, 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, ".rimba"), 0750); err != nil {
		t.Fatal(err)
	}

	cfg := &config.Config{PostCreate: []string{"make test"}}
	if err := trust.Record(dir, trust.Hash(cfg)); err != nil {
		t.Fatal(err)
	}

	cmd, buf := newTrustCmd(cfg, dir)
	cmd.SetArgs([]string{"--show"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("trust --show: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "make test") {
		t.Errorf("--show should display commands, got: %s", out)
	}
	if !strings.Contains(out, "sha256:") {
		t.Errorf("--show should display hash, got: %s", out)
	}
	if !strings.Contains(out, "trusted") {
		t.Errorf("--show should display trusted state, got: %s", out)
	}
}

func TestTrustCmdApproveDecline(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".rimba"), 0750); err != nil {
		t.Fatal(err)
	}

	cfg := &config.Config{PostCreate: []string{"rm -rf /"}}
	cmd, buf := newTrustCmd(cfg, dir)
	cmd.SetIn(strings.NewReader("n\n"))

	if err := cmd.Execute(); err != nil {
		t.Fatalf("trust approve decline: %v", err)
	}

	if !strings.Contains(strings.ToLower(buf.String()), "declined") {
		t.Errorf("expected 'declined' in output, got: %s", buf.String())
	}
	ok, _ := trust.IsTrusted(dir, trust.Hash(cfg))
	if ok {
		t.Error("trust should not be recorded after decline")
	}
}

func TestTrustCmdApproveIsTrustedError(t *testing.T) {
	dir := t.TempDir()
	rimbaDir := filepath.Join(dir, ".rimba")
	if err := os.MkdirAll(rimbaDir, 0750); err != nil {
		t.Fatal(err)
	}
	// Make trust.local.toml a directory so IsTrusted returns an error.
	if err := os.Mkdir(filepath.Join(rimbaDir, "trust.local.toml"), 0750); err != nil {
		t.Fatal(err)
	}

	cfg := &config.Config{PostCreate: []string{"make install"}}
	cmd, _ := newTrustCmd(cfg, dir)
	if err := cmd.Execute(); err == nil {
		t.Error("trust approve with IsTrusted error should fail")
	}
}

func TestTrustCmdApproveRecordError(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".rimba"), 0750); err != nil {
		t.Fatal(err)
	}
	// Place a directory at .gitignore so Record fails on EnsureGitignore.
	if err := os.Mkdir(filepath.Join(dir, ".gitignore"), 0750); err != nil {
		t.Fatal(err)
	}

	cfg := &config.Config{PostCreate: []string{"make install"}}
	cmd, _ := newTrustCmd(cfg, dir)
	cmd.SetIn(strings.NewReader("y\n"))
	if err := cmd.Execute(); err == nil {
		t.Error("trust approve with Record error should fail")
	}
}

func TestTrustCmdShowIsTrustedError(t *testing.T) {
	dir := t.TempDir()
	rimbaDir := filepath.Join(dir, ".rimba")
	if err := os.MkdirAll(rimbaDir, 0750); err != nil {
		t.Fatal(err)
	}
	// Make trust.local.toml a directory so IsTrusted returns an error.
	if err := os.Mkdir(filepath.Join(rimbaDir, "trust.local.toml"), 0750); err != nil {
		t.Fatal(err)
	}

	cfg := &config.Config{PostCreate: []string{"make test"}}
	cmd, _ := newTrustCmd(cfg, dir)
	cmd.SetArgs([]string{"--show"})
	if err := cmd.Execute(); err == nil {
		t.Error("trust --show with IsTrusted error should fail")
	}
}

func TestTrustCmdShowUntrusted(t *testing.T) {
	dir := t.TempDir()
	cfg := &config.Config{PostCreate: []string{"make test"}}
	cmd, buf := newTrustCmd(cfg, dir)
	cmd.SetArgs([]string{"--show"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("trust --show untrusted: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "not trusted") {
		t.Errorf("--show should display 'not trusted', got: %s", out)
	}
}

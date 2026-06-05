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

func newTrustTestCmd() (*cobra.Command, *bytes.Buffer) {
	buf := &bytes.Buffer{}
	cmd := &cobra.Command{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.Flags().Bool(flagYes, false, "")
	return cmd, buf
}

func setupTrustTestRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".gitignore"), []byte(".rimba/settings.local.toml\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, ".rimba"), 0750); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestEnsureTrustEmptyConfig(t *testing.T) {
	dir := setupTrustTestRepo(t)
	cmd, _ := newTrustTestCmd()
	if err := ensureTrust(cmd, dir, &config.Config{}); err != nil {
		t.Errorf("ensureTrust with empty config should return nil, got: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, ".rimba", trust.FileName)); !os.IsNotExist(err) {
		t.Error("no trust file should be written for empty config")
	}
}

func TestEnsureTrustAlreadyTrusted(t *testing.T) {
	dir := setupTrustTestRepo(t)
	cfg := &config.Config{PostCreate: []string{"make test"}}
	h := trust.Hash(cfg)
	if err := trust.Record(dir, h); err != nil {
		t.Fatal(err)
	}

	cmd, buf := newTrustTestCmd()
	if err := ensureTrust(cmd, dir, cfg); err != nil {
		t.Errorf("ensureTrust trusted should return nil, got: %v", err)
	}
	if buf.Len() > 0 {
		t.Errorf("trusted path should not prompt, got output: %q", buf.String())
	}
}

func TestEnsureTrustPromptYes(t *testing.T) {
	dir := setupTrustTestRepo(t)
	cfg := &config.Config{PostCreate: []string{"make install"}}

	cmd, _ := newTrustTestCmd()
	cmd.SetIn(strings.NewReader("y\n"))
	if err := ensureTrust(cmd, dir, cfg); err != nil {
		t.Errorf("prompt 'y' should return nil, got: %v", err)
	}

	ok, err := trust.IsTrusted(dir, trust.Hash(cfg))
	if err != nil || !ok {
		t.Errorf("trust should be recorded after 'y' consent: ok=%v err=%v", ok, err)
	}
}

func TestEnsureTrustPromptNo(t *testing.T) {
	dir := setupTrustTestRepo(t)
	cfg := &config.Config{PostCreate: []string{"rm -rf /"}}

	cmd, _ := newTrustTestCmd()
	cmd.SetIn(strings.NewReader("n\n"))
	err := ensureTrust(cmd, dir, cfg)
	if err == nil {
		t.Error("prompt 'n' should return error")
	}

	ok, _ := trust.IsTrusted(dir, trust.Hash(cfg))
	if ok {
		t.Error("trust should not be recorded after 'n'")
	}
}

func TestEnsureTrustPromptEOFDefaultNo(t *testing.T) {
	dir := setupTrustTestRepo(t)
	cfg := &config.Config{PostCreate: []string{"make test"}}

	cmd, _ := newTrustTestCmd()
	cmd.SetIn(strings.NewReader("")) // EOF
	err := ensureTrust(cmd, dir, cfg)
	if err == nil {
		t.Error("EOF (non-interactive) should return error (default no)")
	}
}

func TestEnsureTrustFlagYes(t *testing.T) {
	dir := setupTrustTestRepo(t)
	cfg := &config.Config{PostCreate: []string{"pnpm install"}}

	cmd, buf := newTrustTestCmd()
	if err := cmd.Flags().Set(flagYes, "true"); err != nil {
		t.Fatal(err)
	}
	if err := ensureTrust(cmd, dir, cfg); err != nil {
		t.Errorf("--yes should return nil, got: %v", err)
	}
	if buf.Len() > 0 {
		t.Errorf("--yes should not prompt, got output: %q", buf.String())
	}
	ok, _ := trust.IsTrusted(dir, trust.Hash(cfg))
	if !ok {
		t.Error("trust should be recorded when --yes is set")
	}
}

func TestEnsureTrustEnvYes(t *testing.T) {
	dir := setupTrustTestRepo(t)
	cfg := &config.Config{PostCreate: []string{"npm ci"}}

	t.Setenv("RIMBA_TRUST_YES", "1")
	cmd, buf := newTrustTestCmd()
	if err := ensureTrust(cmd, dir, cfg); err != nil {
		t.Errorf("RIMBA_TRUST_YES=1 should return nil, got: %v", err)
	}
	if buf.Len() > 0 {
		t.Errorf("env escape hatch should not prompt, got output: %q", buf.String())
	}
	ok, _ := trust.IsTrusted(dir, trust.Hash(cfg))
	if !ok {
		t.Error("trust should be recorded when RIMBA_TRUST_YES=1")
	}
}

func TestEnsureTrustPromptShowsCommands(t *testing.T) {
	dir := setupTrustTestRepo(t)
	cfg := &config.Config{
		PostCreate: []string{"pnpm install", "./setup.sh"},
	}

	cmd, buf := newTrustTestCmd()
	cmd.SetIn(strings.NewReader("n\n"))
	_ = ensureTrust(cmd, dir, cfg)

	out := buf.String()
	if !strings.Contains(out, "pnpm install") {
		t.Errorf("prompt should show commands, got:\n%s", out)
	}
	if !strings.Contains(out, "./setup.sh") {
		t.Errorf("prompt should show all commands, got:\n%s", out)
	}
}

func TestEnsureTrustIsTrustedError(t *testing.T) {
	dir := setupTrustTestRepo(t)
	rimbaDir := filepath.Join(dir, ".rimba")
	// Make trust.local.toml a directory so IsTrusted returns an error.
	if err := os.Mkdir(filepath.Join(rimbaDir, "trust.local.toml"), 0750); err != nil {
		t.Fatal(err)
	}

	cfg := &config.Config{PostCreate: []string{"make test"}}
	cmd, _ := newTrustTestCmd()
	err := ensureTrust(cmd, dir, cfg)
	if err == nil {
		t.Error("ensureTrust should propagate IsTrusted error")
	}
}

func TestEnsureTrustErrorContainsHint(t *testing.T) {
	dir := setupTrustTestRepo(t)
	cfg := &config.Config{PostCreate: []string{"make test"}}

	cmd, _ := newTrustTestCmd()
	cmd.SetIn(strings.NewReader("n\n"))
	err := ensureTrust(cmd, dir, cfg)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "rimba trust") {
		t.Errorf("error should mention 'rimba trust', got: %v", err)
	}
}

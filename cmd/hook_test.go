package cmd

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/lugassawan/rimba/internal/config"
	"github.com/lugassawan/rimba/internal/hook"
)

func hookTestRunner(repoDir string) *mockRunner {
	return &mockRunner{
		run: func(args ...string) (string, error) {
			if len(args) >= 2 && args[1] == "--show-toplevel" {
				return repoDir, nil
			}
			if len(args) >= 2 && args[1] == "--git-common-dir" {
				return filepath.Join(repoDir, ".git"), nil
			}
			if args[0] == cmdSymbolicRef {
				return refsRemotesOriginMain, nil
			}
			return "", nil
		},
		runInDir: noopRunInDir,
	}
}

func TestHookInstallSuccess(t *testing.T) {
	repoDir := t.TempDir()
	cfg := &config.Config{DefaultSource: branchMain}
	_ = config.Save(filepath.Join(repoDir, config.FileName), cfg)

	r := hookTestRunner(repoDir)
	restore := overrideNewRunner(r)
	defer restore()

	cmd, buf := newTestCmd()
	err := hookInstallCmd.RunE(cmd, nil)
	if err != nil {
		t.Fatalf("hookInstallCmd.RunE: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "Installed post-merge hook") {
		t.Errorf("output = %q, want 'Installed post-merge hook'", out)
	}
}

func TestHookInstallAlreadyInstalled(t *testing.T) {
	repoDir := t.TempDir()
	cfg := &config.Config{DefaultSource: branchMain}
	_ = config.Save(filepath.Join(repoDir, config.FileName), cfg)

	hooksDir := filepath.Join(repoDir, ".git", "hooks")
	_ = hook.Install(hooksDir, branchMain)

	r := hookTestRunner(repoDir)
	restore := overrideNewRunner(r)
	defer restore()

	cmd, buf := newTestCmd()
	err := hookInstallCmd.RunE(cmd, nil)
	if err != nil {
		t.Fatalf("hookInstallCmd.RunE: %v", err)
	}
	if !strings.Contains(buf.String(), "already installed") {
		t.Errorf("output = %q, want 'already installed'", buf.String())
	}
}

func TestHookUninstallSuccess(t *testing.T) {
	repoDir := t.TempDir()

	hooksDir := filepath.Join(repoDir, ".git", "hooks")
	_ = hook.Install(hooksDir, branchMain)

	r := hookTestRunner(repoDir)
	restore := overrideNewRunner(r)
	defer restore()

	cmd, buf := newTestCmd()
	err := hookUninstallCmd.RunE(cmd, nil)
	if err != nil {
		t.Fatalf("hookUninstallCmd.RunE: %v", err)
	}
	if !strings.Contains(buf.String(), "Uninstalled") {
		t.Errorf("output = %q, want 'Uninstalled'", buf.String())
	}
}

func TestHookUninstallNotInstalled(t *testing.T) {
	repoDir := t.TempDir()

	r := hookTestRunner(repoDir)
	restore := overrideNewRunner(r)
	defer restore()

	cmd, _ := newTestCmd()
	err := hookUninstallCmd.RunE(cmd, nil)
	if err == nil {
		t.Fatal("expected error for uninstalling when not installed")
	}
	if !strings.Contains(err.Error(), "not installed") {
		t.Errorf("error = %q, want 'not installed'", err.Error())
	}
}

func TestHookStatusInstalled(t *testing.T) {
	repoDir := t.TempDir()

	hooksDir := filepath.Join(repoDir, ".git", "hooks")
	_ = hook.Install(hooksDir, branchMain)

	r := hookTestRunner(repoDir)
	restore := overrideNewRunner(r)
	defer restore()

	cmd, buf := newTestCmd()
	err := hookStatusCmd.RunE(cmd, nil)
	if err != nil {
		t.Fatalf("hookStatusCmd.RunE: %v", err)
	}
	if !strings.Contains(buf.String(), "is installed") {
		t.Errorf("output = %q, want 'is installed'", buf.String())
	}
}

func TestHookStatusNotInstalled(t *testing.T) {
	repoDir := t.TempDir()

	r := hookTestRunner(repoDir)
	restore := overrideNewRunner(r)
	defer restore()

	cmd, buf := newTestCmd()
	err := hookStatusCmd.RunE(cmd, nil)
	if err != nil {
		t.Fatalf("hookStatusCmd.RunE: %v", err)
	}
	if !strings.Contains(buf.String(), "not installed") {
		t.Errorf("output = %q, want 'not installed'", buf.String())
	}
}

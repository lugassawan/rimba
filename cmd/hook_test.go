package cmd

import (
	"os"
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
	if !strings.Contains(out, "Installed pre-commit hook") {
		t.Errorf("output = %q, want 'Installed pre-commit hook'", out)
	}
}

func TestHookInstallAlreadyInstalled(t *testing.T) {
	repoDir := t.TempDir()
	cfg := &config.Config{DefaultSource: branchMain}
	_ = config.Save(filepath.Join(repoDir, config.FileName), cfg)

	hooksDir := filepath.Join(repoDir, ".git", "hooks")
	_ = hook.Install(hooksDir, hook.PostMergeHook, hook.PostMergeBlock(branchMain))
	_ = hook.Install(hooksDir, hook.PreCommitHook, hook.PreCommitBlock())

	r := hookTestRunner(repoDir)
	restore := overrideNewRunner(r)
	defer restore()

	cmd, buf := newTestCmd()
	err := hookInstallCmd.RunE(cmd, nil)
	if err != nil {
		t.Fatalf("hookInstallCmd.RunE: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "post-merge hook is already installed") {
		t.Errorf("output = %q, want 'post-merge hook is already installed'", out)
	}
	if !strings.Contains(out, "pre-commit hook is already installed") {
		t.Errorf("output = %q, want 'pre-commit hook is already installed'", out)
	}
}

func TestHookUninstallSuccess(t *testing.T) {
	repoDir := t.TempDir()

	hooksDir := filepath.Join(repoDir, ".git", "hooks")
	_ = hook.Install(hooksDir, hook.PostMergeHook, hook.PostMergeBlock(branchMain))
	_ = hook.Install(hooksDir, hook.PreCommitHook, hook.PreCommitBlock())

	r := hookTestRunner(repoDir)
	restore := overrideNewRunner(r)
	defer restore()

	cmd, buf := newTestCmd()
	err := hookUninstallCmd.RunE(cmd, nil)
	if err != nil {
		t.Fatalf("hookUninstallCmd.RunE: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "Uninstalled rimba post-merge hook") {
		t.Errorf("output = %q, want 'Uninstalled rimba post-merge hook'", out)
	}
	if !strings.Contains(out, "Uninstalled rimba pre-commit hook") {
		t.Errorf("output = %q, want 'Uninstalled rimba pre-commit hook'", out)
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
	_ = hook.Install(hooksDir, hook.PostMergeHook, hook.PostMergeBlock(branchMain))
	_ = hook.Install(hooksDir, hook.PreCommitHook, hook.PreCommitBlock())

	r := hookTestRunner(repoDir)
	restore := overrideNewRunner(r)
	defer restore()

	cmd, buf := newTestCmd()
	err := hookStatusCmd.RunE(cmd, nil)
	if err != nil {
		t.Fatalf("hookStatusCmd.RunE: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "post-merge hook is installed") {
		t.Errorf("output = %q, want 'post-merge hook is installed'", out)
	}
	if !strings.Contains(out, "pre-commit hook is installed") {
		t.Errorf("output = %q, want 'pre-commit hook is installed'", out)
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
	out := buf.String()
	if !strings.Contains(out, "post-merge hook is not installed") {
		t.Errorf("output = %q, want 'post-merge hook is not installed'", out)
	}
	if !strings.Contains(out, "pre-commit hook is not installed") {
		t.Errorf("output = %q, want 'pre-commit hook is not installed'", out)
	}
}

// --- Corrupt block handling ---

func writeCorruptHook(t *testing.T, repoDir, hookName string) string {
	t.Helper()
	hooksDir := filepath.Join(repoDir, ".git", "hooks")
	if err := os.MkdirAll(hooksDir, 0750); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	hookPath := filepath.Join(hooksDir, hookName)
	corrupt := "#!/bin/sh\n\n" + hook.BeginMarker + "\nmy custom lefthook line\n"
	if err := os.WriteFile(hookPath, []byte(corrupt), 0755); err != nil {
		t.Fatalf("write corrupt hook: %v", err)
	}
	return hookPath
}

func TestHookStatusReportsCorrupt(t *testing.T) {
	repoDir := t.TempDir()
	writeCorruptHook(t, repoDir, hook.PostMergeHook)

	r := hookTestRunner(repoDir)
	restore := overrideNewRunner(r)
	defer restore()

	cmd, buf := newTestCmd()
	err := hookStatusCmd.RunE(cmd, nil)
	if err != nil {
		t.Fatalf("hookStatusCmd.RunE: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "corrupt") {
		t.Errorf("output = %q, want a corruption warning", out)
	}
	if !strings.Contains(out, "resolve manually") {
		t.Errorf("output = %q, want 'resolve manually'", out)
	}
}

func TestHookInstallSurfacesCorrupt(t *testing.T) {
	repoDir := t.TempDir()
	cfg := &config.Config{DefaultSource: branchMain}
	_ = config.Save(filepath.Join(repoDir, config.FileName), cfg)
	hookPath := writeCorruptHook(t, repoDir, hook.PostMergeHook)
	before, _ := os.ReadFile(hookPath)

	r := hookTestRunner(repoDir)
	restore := overrideNewRunner(r)
	defer restore()

	cmd, _ := newTestCmd()
	err := hookInstallCmd.RunE(cmd, nil)
	if err == nil {
		t.Fatal("expected error for corrupt hook file")
	}
	if !strings.Contains(err.Error(), "resolve manually") {
		t.Errorf("error = %q, want 'resolve manually'", err.Error())
	}
	if !strings.Contains(err.Error(), hookPath) {
		t.Errorf("error = %q, want to contain hook path %q", err.Error(), hookPath)
	}

	after, _ := os.ReadFile(hookPath)
	if string(after) != string(before) {
		t.Errorf("hook file should be unchanged, got %q, want %q", after, before)
	}
}

func TestHookUninstallSurfacesCorrupt(t *testing.T) {
	repoDir := t.TempDir()
	hookPath := writeCorruptHook(t, repoDir, hook.PostMergeHook)
	before, _ := os.ReadFile(hookPath)

	r := hookTestRunner(repoDir)
	restore := overrideNewRunner(r)
	defer restore()

	cmd, _ := newTestCmd()
	err := hookUninstallCmd.RunE(cmd, nil)
	if err == nil {
		t.Fatal("expected error for corrupt hook file")
	}
	if !strings.Contains(err.Error(), "resolve manually") {
		t.Errorf("error = %q, want 'resolve manually'", err.Error())
	}
	if !strings.Contains(err.Error(), hookPath) {
		t.Errorf("error = %q, want to contain hook path %q", err.Error(), hookPath)
	}

	after, _ := os.ReadFile(hookPath)
	if string(after) != string(before) {
		t.Errorf("hook file should be unchanged, got %q, want %q", after, before)
	}
}

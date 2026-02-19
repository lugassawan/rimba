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

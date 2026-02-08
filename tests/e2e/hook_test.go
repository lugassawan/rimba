package e2e_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lugassawan/rimba/internal/hook"
	"github.com/lugassawan/rimba/testutil"
)

const (
	hookFileName   = hook.HookName
	fatalReadHook  = "read hook: %v"
)

func TestHookInstall(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupRepo(t)

	r := rimbaSuccess(t, repo, "hook", "install")
	assertContains(t, r.Stdout, "Installed post-merge hook")

	// Verify file exists and is executable
	hookPath := filepath.Join(repo, ".git", "hooks", hookFileName)
	info, err := os.Stat(hookPath)
	if err != nil {
		t.Fatalf("hook file should exist: %v", err)
	}
	if info.Mode().Perm()&0111 == 0 {
		t.Error("hook file should be executable")
	}

	// Verify content has rimba markers
	content, err := os.ReadFile(hookPath)
	if err != nil {
		t.Fatalf(fatalReadHook, err)
	}
	if !strings.Contains(string(content), hook.BeginMarker) {
		t.Error("hook file should contain BEGIN marker")
	}
	if !strings.Contains(string(content), hook.EndMarker) {
		t.Error("hook file should contain END marker")
	}
}

func TestHookInstallIdempotent(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupRepo(t)

	rimbaSuccess(t, repo, "hook", "install")
	// Second install should not fail
	r := rimbaSuccess(t, repo, "hook", "install")
	assertContains(t, r.Stdout, "already installed")
}

func TestHookUninstall(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupRepo(t)

	rimbaSuccess(t, repo, "hook", "install")
	r := rimbaSuccess(t, repo, "hook", "uninstall")
	assertContains(t, r.Stdout, "Uninstalled")

	hookPath := filepath.Join(repo, ".git", "hooks", hookFileName)
	if _, err := os.Stat(hookPath); !os.IsNotExist(err) {
		t.Error("hook file should be removed after uninstall")
	}
}

func TestHookUninstallNotInstalled(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupRepo(t)
	r := rimbaFail(t, repo, "hook", "uninstall")
	assertContains(t, r.Stderr, "not installed")
}

func TestHookStatus(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupRepo(t)

	// Before install
	r := rimbaSuccess(t, repo, "hook", "status")
	assertContains(t, r.Stdout, "not installed")

	// After install
	rimbaSuccess(t, repo, "hook", "install")
	r = rimbaSuccess(t, repo, "hook", "status")
	assertContains(t, r.Stdout, "is installed")
}

func TestHookPreservesExistingHook(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupRepo(t)

	// Create a user hook first
	hooksDir := filepath.Join(repo, ".git", "hooks")
	if err := os.MkdirAll(hooksDir, 0755); err != nil {
		t.Fatalf("mkdir hooks: %v", err)
	}
	userContent := "#!/bin/sh\necho 'user hook'\n"
	if err := os.WriteFile(filepath.Join(hooksDir, hookFileName), []byte(userContent), 0755); err != nil {
		t.Fatalf("write user hook: %v", err)
	}

	// Install rimba hook
	rimbaSuccess(t, repo, "hook", "install")

	content, err := os.ReadFile(filepath.Join(hooksDir, hookFileName))
	if err != nil {
		t.Fatalf(fatalReadHook, err)
	}
	s := string(content)

	// Both should be present
	if !strings.Contains(s, "echo 'user hook'") {
		t.Error("user hook content should be preserved after install")
	}
	if !strings.Contains(s, hook.BeginMarker) {
		t.Error("rimba block should be present after install")
	}

	// Uninstall rimba hook
	rimbaSuccess(t, repo, "hook", "uninstall")

	content, err = os.ReadFile(filepath.Join(hooksDir, hookFileName))
	if err != nil {
		t.Fatalf("read hook after uninstall: %v", err)
	}
	s = string(content)

	// User content preserved, rimba removed
	if !strings.Contains(s, "echo 'user hook'") {
		t.Error("user hook content should be preserved after uninstall")
	}
	if strings.Contains(s, hook.BeginMarker) {
		t.Error("rimba block should be removed after uninstall")
	}
}

func TestHookWorksWithoutInit(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupRepo(t) // no rimba init
	rimbaSuccess(t, repo, "hook", "install")
	rimbaSuccess(t, repo, "hook", "status")
	rimbaSuccess(t, repo, "hook", "uninstall")
}

func TestHookPostMergeRuns(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupRepo(t)
	rimbaSuccess(t, repo, "hook", "install")

	// Create a branch, commit, and merge to trigger the hook
	testutil.GitCmd(t, repo, "checkout", "-b", "test-branch")
	testutil.CreateFile(t, repo, "hook-test.txt", "hook test content")
	testutil.GitCmd(t, repo, "add", ".")
	testutil.GitCmd(t, repo, "commit", "-m", "hook test commit")
	testutil.GitCmd(t, repo, "checkout", "main")

	// Merge should succeed (hook should not break it)
	testutil.GitCmd(t, repo, "merge", "test-branch")

	// Verify the merge worked (file exists)
	assertFileExists(t, filepath.Join(repo, "hook-test.txt"))
}

package e2e_test

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lugassawan/rimba/internal/hook"
	"github.com/lugassawan/rimba/testutil"
)

const fatalReadHook = "read hook: %v"

// resolveHooksDir returns the hooks directory for a repo, respecting core.hooksPath.
func resolveHooksDir(t *testing.T, repo string) string {
	t.Helper()
	cmd := exec.Command("git", "config", "core.hooksPath")
	cmd.Dir = repo
	out, err := cmd.Output()
	if err == nil {
		p := strings.TrimSpace(string(out))
		if p != "" {
			if filepath.IsAbs(p) {
				return p
			}
			return filepath.Join(repo, p)
		}
	}
	return filepath.Join(repo, ".git", "hooks")
}

func TestHookInstall(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupRepo(t)

	r := rimbaSuccess(t, repo, "hook", "install")
	assertContains(t, r.Stdout, "Installed post-merge hook")
	assertContains(t, r.Stdout, "Installed pre-commit hook")

	// Verify post-merge file exists and is executable
	hooksDir := resolveHooksDir(t, repo)
	postMergePath := filepath.Join(hooksDir, hook.PostMergeHook)
	info, err := os.Stat(postMergePath)
	if err != nil {
		t.Fatalf("post-merge hook file should exist: %v", err)
	}
	if info.Mode().Perm()&0111 == 0 {
		t.Error("post-merge hook file should be executable")
	}

	// Verify pre-commit file exists and is executable
	preCommitPath := filepath.Join(hooksDir, hook.PreCommitHook)
	info, err = os.Stat(preCommitPath)
	if err != nil {
		t.Fatalf("pre-commit hook file should exist: %v", err)
	}
	if info.Mode().Perm()&0111 == 0 {
		t.Error("pre-commit hook file should be executable")
	}

	// Verify content has rimba markers
	for _, hookPath := range []string{postMergePath, preCommitPath} {
		content, err := os.ReadFile(hookPath)
		if err != nil {
			t.Fatalf(fatalReadHook, err)
		}
		if !strings.Contains(string(content), hook.BeginMarker) {
			t.Errorf("%s should contain BEGIN marker", hookPath)
		}
		if !strings.Contains(string(content), hook.EndMarker) {
			t.Errorf("%s should contain END marker", hookPath)
		}
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
	assertContains(t, r.Stdout, "Uninstalled rimba post-merge hook")
	assertContains(t, r.Stdout, "Uninstalled rimba pre-commit hook")

	hooksDir := resolveHooksDir(t, repo)
	postMergePath := filepath.Join(hooksDir, hook.PostMergeHook)
	if _, err := os.Stat(postMergePath); !os.IsNotExist(err) {
		t.Error("post-merge hook file should be removed after uninstall")
	}

	preCommitPath := filepath.Join(hooksDir, hook.PreCommitHook)
	if _, err := os.Stat(preCommitPath); !os.IsNotExist(err) {
		t.Error("pre-commit hook file should be removed after uninstall")
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
	assertContains(t, r.Stdout, "post-merge hook is not installed")
	assertContains(t, r.Stdout, "pre-commit hook is not installed")

	// After install
	rimbaSuccess(t, repo, "hook", "install")
	r = rimbaSuccess(t, repo, "hook", "status")
	assertContains(t, r.Stdout, "post-merge hook is installed")
	assertContains(t, r.Stdout, "pre-commit hook is installed")
}

func TestHookPreservesExistingHook(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupRepo(t)

	// Create a user hook in post-merge first
	hooksDir := resolveHooksDir(t, repo)
	if err := os.MkdirAll(hooksDir, 0755); err != nil {
		t.Fatalf("mkdir hooks: %v", err)
	}
	userContent := "#!/bin/sh\necho 'user hook'\n"
	if err := os.WriteFile(filepath.Join(hooksDir, hook.PostMergeHook), []byte(userContent), 0755); err != nil {
		t.Fatalf("write user hook: %v", err)
	}

	// Install rimba hook
	rimbaSuccess(t, repo, "hook", "install")

	content, err := os.ReadFile(filepath.Join(hooksDir, hook.PostMergeHook))
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

	// Uninstall rimba hooks
	rimbaSuccess(t, repo, "hook", "uninstall")

	content, err = os.ReadFile(filepath.Join(hooksDir, hook.PostMergeHook))
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

func TestHookInstallWithCoreHooksPath(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupRepo(t)

	// Set core.hooksPath to .githooks
	testutil.GitCmd(t, repo, "config", "core.hooksPath", ".githooks")

	// Install hooks — should go into .githooks/, not .git/hooks/
	rimbaSuccess(t, repo, "hook", "install")

	customDir := filepath.Join(repo, ".githooks")
	postMergePath := filepath.Join(customDir, hook.PostMergeHook)
	if _, err := os.Stat(postMergePath); err != nil {
		t.Fatalf("post-merge hook should exist in .githooks/: %v", err)
	}
	preCommitPath := filepath.Join(customDir, hook.PreCommitHook)
	if _, err := os.Stat(preCommitPath); err != nil {
		t.Fatalf("pre-commit hook should exist in .githooks/: %v", err)
	}

	// Verify hooks are NOT in .git/hooks/
	defaultDir := filepath.Join(repo, ".git", "hooks")
	if _, err := os.Stat(filepath.Join(defaultDir, hook.PostMergeHook)); err == nil {
		t.Error("post-merge hook should NOT exist in .git/hooks/ when core.hooksPath is set")
	}

	// Status should report installed
	r := rimbaSuccess(t, repo, "hook", "status")
	assertContains(t, r.Stdout, "post-merge hook is installed")
	assertContains(t, r.Stdout, "pre-commit hook is installed")

	// Uninstall should remove from .githooks/
	rimbaSuccess(t, repo, "hook", "uninstall")
	if _, err := os.Stat(postMergePath); !os.IsNotExist(err) {
		t.Error("post-merge hook should be removed from .githooks/ after uninstall")
	}
	if _, err := os.Stat(preCommitPath); !os.IsNotExist(err) {
		t.Error("pre-commit hook should be removed from .githooks/ after uninstall")
	}
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

func TestHookPreCommitBlocksMainCommit(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupRepo(t)
	rimbaSuccess(t, repo, "hook", "install")

	// Try to commit on main — should be blocked
	testutil.CreateFile(t, repo, "blocked.txt", "should not commit")
	testutil.GitCmd(t, repo, "add", ".")

	// Run git commit directly — expect failure
	r := gitCommitResult(t, repo, "blocked commit on main")
	if r.ExitCode == 0 {
		t.Fatal("git commit on main should fail with pre-commit hook installed")
	}
	if !strings.Contains(r.Stderr, "direct commits to main are not allowed") &&
		!strings.Contains(r.Stdout, "direct commits to main are not allowed") {
		t.Errorf("expected protection message, got:\nstdout: %s\nstderr: %s", r.Stdout, r.Stderr)
	}
}

func TestHookPreCommitAllowsBranchCommit(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupRepo(t)
	rimbaSuccess(t, repo, "hook", "install")

	// Switch to a feature branch
	testutil.GitCmd(t, repo, "checkout", "-b", "feature/test-branch")
	testutil.CreateFile(t, repo, "allowed.txt", "should commit fine")
	testutil.GitCmd(t, repo, "add", ".")

	// Commit should succeed on a feature branch
	r := gitCommitResult(t, repo, "allowed commit on branch")
	if r.ExitCode != 0 {
		t.Fatalf("git commit on feature branch should succeed, got exit %d\nstdout: %s\nstderr: %s",
			r.ExitCode, r.Stdout, r.Stderr)
	}
}

// gitCommitResult runs `git commit` in the repo and returns the result.
func gitCommitResult(t *testing.T, dir, msg string) result {
	t.Helper()

	cmd := exec.Command("git", "commit", "-m", msg)
	cmd.Dir = dir

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	r := result{
		Stdout: stdout.String(),
		Stderr: stderr.String(),
	}

	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			r.ExitCode = exitErr.ExitCode()
		} else {
			t.Fatalf("failed to run git commit: %v", err)
		}
	}

	return r
}

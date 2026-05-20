package e2e_test

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lugassawan/rimba/testutil"
)

// conflictRow describes one conflict scenario for TestSetupConflicts.
type conflictRow struct {
	name  string
	setup func(t *testing.T, repo string)
}

// resolveScriptPath returns the absolute path to scripts/setup.sh, relative to
// tests/e2e/ where this file lives.
func resolveScriptPath(t *testing.T) string {
	t.Helper()
	p, err := filepath.Abs(filepath.Join("..", "..", "scripts", "setup.sh"))
	if err != nil {
		t.Fatalf("resolve script path: %v", err)
	}
	if _, err := os.Stat(p); err != nil {
		t.Fatalf("scripts/setup.sh not found at %s: %v", p, err)
	}
	return p
}

// runSetup executes scripts/setup.sh inside repo with optional args, mirrors
// the result/rimba helper pattern in e2e_test.go.
func runSetup(t *testing.T, repo string, args ...string) result {
	t.Helper()
	scriptPath := resolveScriptPath(t)
	cmdArgs := append([]string{scriptPath}, args...)
	cmd := exec.Command("sh", cmdArgs...)
	cmd.Dir = repo

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
			t.Fatalf("failed to run setup.sh: %v", err)
		}
	}
	return r
}

// populateGithooks creates the three standard hook source files in
// repo/.githooks/ so setup.sh has something to symlink.
func populateGithooks(t *testing.T, repo string) {
	t.Helper()
	dir := filepath.Join(repo, ".githooks")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir .githooks: %v", err)
	}
	for _, name := range []string{"pre-commit", "commit-msg", "post-merge"} {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte("#!/bin/sh\n"), 0o755); err != nil {
			t.Fatalf("write .githooks/%s: %v", name, err)
		}
	}
}

// repoRealPath returns the canonical path of the repo, resolving any OS-level
// symlinks (e.g. /var -> /private/var on macOS) so it matches what
// `git rev-parse --show-toplevel` returns inside scripts.
func repoRealPath(t *testing.T, repo string) string {
	t.Helper()
	canonical, err := filepath.EvalSymlinks(repo)
	if err != nil {
		t.Fatalf("eval symlinks %s: %v", repo, err)
	}
	return canonical
}

func TestSetupFreshInstall(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupRepo(t)
	populateGithooks(t, repo)

	r := runSetup(t, repo)
	if r.ExitCode != 0 {
		t.Fatalf("expected exit 0, got %d\nstdout: %s\nstderr: %s", r.ExitCode, r.Stdout, r.Stderr)
	}

	repoReal := repoRealPath(t, repo)
	gitDir := strings.TrimSpace(testutil.GitCmd(t, repo, "rev-parse", "--git-dir"))
	if !filepath.IsAbs(gitDir) {
		gitDir = filepath.Join(repoReal, gitDir)
	}

	for _, name := range []string{"pre-commit", "commit-msg", "post-merge"} {
		linkPath := filepath.Join(gitDir, "hooks", name)
		target, err := os.Readlink(linkPath)
		if err != nil {
			t.Errorf("%s: expected symlink, got error: %v", linkPath, err)
			continue
		}
		expected := filepath.Join(repoReal, ".githooks", name)
		if target != expected {
			t.Errorf("%s -> %q, want %q", linkPath, target, expected)
		}
	}
}

func TestSetupIdempotent(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupRepo(t)
	populateGithooks(t, repo)

	r := runSetup(t, repo)
	if r.ExitCode != 0 {
		t.Fatalf("first run: expected exit 0, got %d\nstderr: %s", r.ExitCode, r.Stderr)
	}

	r2 := runSetup(t, repo)
	if r2.ExitCode != 0 {
		t.Fatalf("second run: expected exit 0, got %d\nstdout: %s\nstderr: %s", r2.ExitCode, r2.Stdout, r2.Stderr)
	}
	if r2.Stderr != "" {
		t.Errorf("second run: expected no stderr, got: %s", r2.Stderr)
	}

	// Symlinks still point at the right targets
	repoReal := repoRealPath(t, repo)
	gitDir := strings.TrimSpace(testutil.GitCmd(t, repo, "rev-parse", "--git-dir"))
	if !filepath.IsAbs(gitDir) {
		gitDir = filepath.Join(repoReal, gitDir)
	}
	for _, name := range []string{"pre-commit", "commit-msg", "post-merge"} {
		linkPath := filepath.Join(gitDir, "hooks", name)
		if _, err := os.Lstat(linkPath); err != nil {
			t.Errorf("%s: symlink missing after second run: %v", linkPath, err)
		}
	}
}

func TestSetupHelp(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupRepo(t)
	r := runSetup(t, repo, "--help")
	if r.ExitCode != 0 {
		t.Fatalf("--help: expected exit 0, got %d\nstdout: %s\nstderr: %s", r.ExitCode, r.Stdout, r.Stderr)
	}
	assertContains(t, r.Stdout, "Usage:")
	assertContains(t, r.Stdout, "--force")
}

func TestSetupUnknownArg(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupRepo(t)
	r := runSetup(t, repo, "--bogus")
	if r.ExitCode != 2 {
		t.Fatalf("--bogus: expected exit 2, got %d\nstdout: %s\nstderr: %s", r.ExitCode, r.Stdout, r.Stderr)
	}
	assertContains(t, r.Stderr, "unknown argument")
}

func TestSetupAllowsExistingHooksPathEqualToGithooks(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupRepo(t)
	populateGithooks(t, repo)
	testutil.GitCmd(t, repo, "config", "core.hooksPath", ".githooks")

	r := runSetup(t, repo)
	if r.ExitCode != 0 {
		t.Fatalf("expected exit 0 with core.hooksPath=.githooks, got %d\nstdout: %s\nstderr: %s",
			r.ExitCode, r.Stdout, r.Stderr)
	}
	if strings.Contains(r.Stderr, "conflict") || strings.Contains(r.Stderr, "Error") {
		t.Errorf("expected no conflict for core.hooksPath=.githooks, got stderr: %s", r.Stderr)
	}
}

func TestSetupConflicts(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	rows := []conflictRow{
		{
			name: "custom core.hooksPath",
			setup: func(t *testing.T, repo string) {
				t.Helper()
				testutil.GitCmd(t, repo, "config", "core.hooksPath", "custom-hooks")
			},
		},
		{
			name: "regular file at hook path",
			setup: func(t *testing.T, repo string) {
				t.Helper()
				hooksDir := filepath.Join(repo, ".git", "hooks")
				if err := os.MkdirAll(hooksDir, 0o755); err != nil {
					t.Fatalf("mkdir: %v", err)
				}
				if err := os.WriteFile(filepath.Join(hooksDir, "pre-commit"), []byte("user content"), 0o755); err != nil {
					t.Fatalf("write: %v", err)
				}
			},
		},
		{
			name: "symlink pointing elsewhere",
			setup: func(t *testing.T, repo string) {
				t.Helper()
				hooksDir := filepath.Join(repo, ".git", "hooks")
				if err := os.MkdirAll(hooksDir, 0o755); err != nil {
					t.Fatalf("mkdir: %v", err)
				}
				if err := os.Symlink("/tmp/elsewhere", filepath.Join(hooksDir, "pre-commit")); err != nil {
					t.Fatalf("symlink: %v", err)
				}
			},
		},
	}

	for _, row := range rows {
		t.Run(row.name, func(t *testing.T) {
			// Without --force: expect exit 1 and a conflict description
			repo := setupRepo(t)
			populateGithooks(t, repo)
			row.setup(t, repo)

			r := runSetup(t, repo)
			if r.ExitCode != 1 {
				t.Fatalf("without --force: expected exit 1, got %d\nstdout: %s\nstderr: %s",
					r.ExitCode, r.Stdout, r.Stderr)
			}
			if r.Stderr == "" {
				t.Error("without --force: expected conflict description on stderr, got nothing")
			}

			// With --force: expect exit 0, overwrite warning, and correct symlink
			repo2 := setupRepo(t)
			populateGithooks(t, repo2)
			row.setup(t, repo2)

			r2 := runSetup(t, repo2, "--force")
			if r2.ExitCode != 0 {
				t.Fatalf("with --force: expected exit 0, got %d\nstdout: %s\nstderr: %s",
					r2.ExitCode, r2.Stdout, r2.Stderr)
			}
			assertContains(t, r2.Stderr, "--force")

			repo2Real := repoRealPath(t, repo2)
			gitDir := strings.TrimSpace(testutil.GitCmd(t, repo2, "rev-parse", "--git-dir"))
			if !filepath.IsAbs(gitDir) {
				gitDir = filepath.Join(repo2Real, gitDir)
			}
			linkPath := filepath.Join(gitDir, "hooks", "pre-commit")
			target, err := os.Readlink(linkPath)
			if err != nil {
				t.Fatalf("with --force: pre-commit symlink missing: %v", err)
			}
			expected := filepath.Join(repo2Real, ".githooks", "pre-commit")
			if target != expected {
				t.Errorf("with --force: pre-commit -> %q, want %q", target, expected)
			}
		})
	}
}

func TestSetupInWorktree(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	mainRepo := setupRepo(t)

	// Add a linked worktree
	wtPath := filepath.Join(t.TempDir(), "linked-wt")
	testutil.GitCmd(t, mainRepo, "worktree", "add", wtPath, "-b", "feature/wt-setup-test")

	// Populate .githooks/ in the WORKTREE's working directory. When the script
	// runs inside the worktree, ROOT = git rev-parse --show-toplevel = wtPath
	// (linked worktrees have their own show-toplevel). This mirrors rimba's
	// behavior: rimba add copies tracked files (including .githooks/) into each
	// linked worktree. Tests that use plain `git worktree add` without that copy
	// step would find no hooks to install — a no-op, not a failure.
	// This test specifically validates GIT_DIR path normalization: symlinks must
	// land under the WORKTREE's own git dir, not the main repo's .git/hooks/.
	populateGithooks(t, wtPath)

	// Run setup inside the linked worktree
	r := runSetup(t, wtPath)
	if r.ExitCode != 0 {
		t.Fatalf("worktree: expected exit 0, got %d\nstdout: %s\nstderr: %s", r.ExitCode, r.Stdout, r.Stderr)
	}

	// Symlinks should be under the worktree's own git dir (not the main repo's
	// .git/hooks/), validating the GIT_DIR absolute-path normalization in setup.sh.
	wtPathReal := repoRealPath(t, wtPath)
	wtGitDir := strings.TrimSpace(testutil.GitCmd(t, wtPath, "rev-parse", "--git-dir"))
	if !filepath.IsAbs(wtGitDir) {
		wtGitDir = filepath.Join(wtPathReal, wtGitDir)
	}

	for _, name := range []string{"pre-commit", "commit-msg", "post-merge"} {
		linkPath := filepath.Join(wtGitDir, "hooks", name)
		target, err := os.Readlink(linkPath)
		if err != nil {
			t.Errorf("worktree %s: expected symlink, got error: %v", linkPath, err)
			continue
		}
		// Target is relative to the worktree root, since ROOT = wtPath.
		expected := filepath.Join(wtPathReal, ".githooks", name)
		if target != expected {
			t.Errorf("worktree %s -> %q, want %q", linkPath, target, expected)
		}
	}
}

func TestSetupHookSourcesAreExecutable(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}
	// Checks that tracked .githooks/ sources have the execute bit set.
	projRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("resolve project root: %v", err)
	}
	githooksDir := filepath.Join(projRoot, ".githooks")
	entries, err := os.ReadDir(githooksDir)
	if err != nil {
		t.Fatalf("read .githooks/: %v", err)
	}
	if len(entries) == 0 {
		t.Fatal(".githooks/ is empty — no hooks to check")
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			t.Fatalf(".githooks/%s: stat: %v", entry.Name(), err)
		}
		if info.Mode().Perm()&0o111 == 0 {
			t.Errorf(".githooks/%s is not executable (mode %s)", entry.Name(), info.Mode().Perm())
		}
	}
}

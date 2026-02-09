package e2e_test

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestOpenPrintsPath(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupInitializedRepo(t)
	rimbaSuccess(t, repo, "add", "open-path")

	r := rimbaSuccess(t, repo, "open", "open-path")

	cfg := loadConfig(t, repo)
	expectedDir := filepath.Join(repo, cfg.WorktreeDir)
	if !strings.Contains(r.Stdout, expectedDir) {
		t.Errorf("expected stdout to contain worktree dir %q, got: %s", expectedDir, r.Stdout)
	}
}

func TestOpenRunsCommand(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupInitializedRepo(t)
	rimbaSuccess(t, repo, "add", "open-cmd")

	r := rimbaSuccess(t, repo, "open", "open-cmd", "pwd")

	// Resolve symlinks — macOS /tmp → /private/tmp
	actual := strings.TrimSpace(r.Stdout)
	actualResolved, err := filepath.EvalSymlinks(actual)
	if err != nil {
		t.Fatalf("EvalSymlinks(%q): %v", actual, err)
	}

	cfg := loadConfig(t, repo)
	expectedDir := filepath.Join(repo, cfg.WorktreeDir)
	expectedResolved, err := filepath.EvalSymlinks(expectedDir)
	if err != nil {
		t.Fatalf("EvalSymlinks(%q): %v", expectedDir, err)
	}

	if !strings.Contains(actualResolved, expectedResolved) {
		t.Errorf("expected pwd output to contain %q, got: %s", expectedResolved, actualResolved)
	}
}

func TestOpenCommandExitCode(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupInitializedRepo(t)
	rimbaSuccess(t, repo, "add", "open-exit")

	r := rimba(t, repo, "open", "open-exit", "false")
	if r.ExitCode == 0 {
		t.Error("expected non-zero exit code from 'false' command")
	}
}

func TestOpenFailsNonexistentTask(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupInitializedRepo(t)
	r := rimbaFail(t, repo, "open", "ghost-task")
	assertContains(t, r.Stderr, "not found")
}

func TestOpenFailsNoArgs(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupInitializedRepo(t)
	rimbaFail(t, repo, "open")
}

func TestOpenFailsCommandNotFound(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupInitializedRepo(t)
	rimbaSuccess(t, repo, "add", "open-notfound")

	r := rimbaFail(t, repo, "open", "open-notfound", "nonexistent-cmd-xyz")
	assertContains(t, r.Stderr, "nonexistent-cmd-xyz")
}

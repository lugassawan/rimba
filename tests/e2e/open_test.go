package e2e_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/lugassawan/rimba/internal/config"
)

const (
	fatalEvalSymlinks = "EvalSymlinks(%q): %v"
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
	assertPwdInWorktreeDir(t, repo, r.Stdout)
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

// setupOpenShortcuts configures the [open] section in a repo's config.
func setupOpenShortcuts(t *testing.T, repo string, shortcuts map[string]string) {
	t.Helper()
	cfg := loadConfig(t, repo)
	cfg.Open = shortcuts
	if err := config.Save(filepath.Join(repo, configFile), cfg); err != nil {
		t.Fatalf(msgSaveConfig, err)
	}
}

// assertPwdInWorktreeDir verifies that pwd output (from stdout) resolves
// to a path inside the repo's worktree directory, handling macOS symlinks.
func assertPwdInWorktreeDir(t *testing.T, repo, stdout string) {
	t.Helper()
	actual := strings.TrimSpace(stdout)
	actualResolved, err := filepath.EvalSymlinks(actual)
	if err != nil {
		t.Fatalf(fatalEvalSymlinks, actual, err)
	}

	cfg := loadConfig(t, repo)
	expectedDir := filepath.Join(repo, cfg.WorktreeDir)
	expectedResolved, err := filepath.EvalSymlinks(expectedDir)
	if err != nil {
		t.Fatalf(fatalEvalSymlinks, expectedDir, err)
	}

	if !strings.Contains(actualResolved, expectedResolved) {
		t.Errorf("expected pwd output to contain %q, got: %s", expectedResolved, actualResolved)
	}
}

func TestOpenWithShortcut(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupInitializedRepo(t)
	setupOpenShortcuts(t, repo, map[string]string{"test": "pwd"})
	rimbaSuccess(t, repo, "add", "open-with")

	r := rimbaSuccess(t, repo, "open", "open-with", "-w", "test")
	assertPwdInWorktreeDir(t, repo, r.Stdout)
}

func TestOpenIDEShortcut(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupInitializedRepo(t)
	setupOpenShortcuts(t, repo, map[string]string{"ide": "pwd"})
	rimbaSuccess(t, repo, "add", "open-ide")

	r := rimbaSuccess(t, repo, "open", "open-ide", "--ide")
	assertPwdInWorktreeDir(t, repo, r.Stdout)
}

func TestOpenAgentShortcut(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupInitializedRepo(t)
	setupOpenShortcuts(t, repo, map[string]string{"agent": "pwd"})
	rimbaSuccess(t, repo, "add", "open-agent")

	r := rimbaSuccess(t, repo, "open", "open-agent", "--agent")
	assertPwdInWorktreeDir(t, repo, r.Stdout)
}

func TestOpenShortcutNotConfigured(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupInitializedRepo(t)
	setupOpenShortcuts(t, repo, map[string]string{"ide": "pwd"})
	rimbaSuccess(t, repo, "add", "open-nosc")

	r := rimbaFail(t, repo, "open", "open-nosc", "-w", "missing")
	assertContains(t, r.Stderr, "not found")
	assertContains(t, r.Stderr, "ide")
}

func TestOpenNoOpenSection(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupInitializedRepo(t)
	rimbaSuccess(t, repo, "add", "open-nosec")

	r := rimbaFail(t, repo, "open", "open-nosec", "--ide")
	assertContains(t, r.Stderr, "no [open] section")
}

func TestOpenShortcutExitCode(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupInitializedRepo(t)
	setupOpenShortcuts(t, repo, map[string]string{"fail": "false"})
	rimbaSuccess(t, repo, "add", "open-sc-exit")

	r := rimba(t, repo, "open", "open-sc-exit", "-w", "fail")
	if r.ExitCode == 0 {
		t.Error("expected non-zero exit code from shortcut running 'false'")
	}
}

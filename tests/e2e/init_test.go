package e2e_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lugassawan/rimba/internal/config"
)

func TestInitCreatesConfigAndDir(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupRepo(t)
	r := rimbaSuccess(t, repo, "init")

	assertContains(t, r.Stdout, "Initialized rimba")
	assertFileExists(t, filepath.Join(repo, configFile))

	// Worktree dir is created relative to repo root
	cfg, err := config.Load(filepath.Join(repo, configFile))
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}
	wtDir := filepath.Join(repo, cfg.WorktreeDir)
	assertFileExists(t, wtDir)

	// .gitignore is created with .rimba.toml
	assertFileExists(t, filepath.Join(repo, gitignoreFile))
	assertGitignoreContains(t, repo, configFile)
}

func TestInitConfigDefaults(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupRepo(t)
	rimbaSuccess(t, repo, "init")

	cfg, err := config.Load(filepath.Join(repo, configFile))
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	if cfg.DefaultSource != "main" {
		t.Errorf("expected default_source %q, got %q", "main", cfg.DefaultSource)
	}
	if len(cfg.CopyFiles) == 0 {
		t.Errorf("expected copy_files to be non-empty")
	}
}

func TestInitExistingConfigInstallsAgentFiles(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupRepo(t)
	rimbaSuccess(t, repo, "init", "--agent-files")

	// Re-running init should succeed and update agent files
	r := rimbaSuccess(t, repo, "init", "--agent-files")
	assertContains(t, r.Stdout, "already exists")
	assertContains(t, r.Stdout, "Agent:")
}

func TestInitAddsToGitignore(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupRepo(t)
	// Pre-create a .gitignore with other entries
	if err := os.WriteFile(filepath.Join(repo, gitignoreFile), []byte("node_modules\n"), 0644); err != nil {
		t.Fatalf("failed to write %s: %v", gitignoreFile, err)
	}

	r := rimbaSuccess(t, repo, "init")
	assertContains(t, r.Stdout, configFile+" added to .gitignore")
	assertGitignoreContains(t, repo, configFile)

	// Original content is preserved
	data, err := os.ReadFile(filepath.Join(repo, gitignoreFile))
	if err != nil {
		t.Fatalf("failed to read %s: %v", gitignoreFile, err)
	}
	if !strings.Contains(string(data), "node_modules") {
		t.Error("expected .gitignore to still contain node_modules")
	}
}

func TestInitGitignoreIdempotent(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupRepo(t)
	// Pre-create .gitignore already containing the entry
	if err := os.WriteFile(filepath.Join(repo, gitignoreFile), []byte(configFile+"\n"), 0644); err != nil {
		t.Fatalf("failed to write %s: %v", gitignoreFile, err)
	}

	r := rimbaSuccess(t, repo, "init")
	assertContains(t, r.Stdout, "already in .gitignore")

	// Verify no duplicate
	data, err := os.ReadFile(filepath.Join(repo, gitignoreFile))
	if err != nil {
		t.Fatalf("failed to read %s: %v", gitignoreFile, err)
	}
	if strings.Count(string(data), configFile) != 1 {
		t.Errorf("expected exactly one %s entry, got:\n%s", configFile, string(data))
	}
}

func TestInitFailsOutsideGitRepo(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	dir := t.TempDir() // not a git repo
	rimbaFail(t, dir, "init")
}

func TestInitWithAgentFilesFlag(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupRepo(t)
	r := rimbaSuccess(t, repo, "init", "--agent-files")

	assertContains(t, r.Stdout, "Agent:")
	assertFileExists(t, filepath.Join(repo, "AGENTS.md"))
	assertFileExists(t, filepath.Join(repo, ".github", "copilot-instructions.md"))
	assertFileExists(t, filepath.Join(repo, ".cursor", "rules", "rimba.mdc"))
	assertFileExists(t, filepath.Join(repo, ".claude", "skills", "rimba", "SKILL.md"))
}

func TestInitSkipsAgentFilesWithoutFlag(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupRepo(t)
	r := rimbaSuccess(t, repo, "init")

	assertNotContains(t, r.Stdout, "Agent:")
	assertFileNotExists(t, filepath.Join(repo, "AGENTS.md"))
}

// assertGitignoreContains verifies that .gitignore in the repo contains the given entry.
func assertGitignoreContains(t *testing.T, repo, entry string) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(repo, gitignoreFile))
	if err != nil {
		t.Fatalf("failed to read .gitignore: %v", err)
	}
	for line := range strings.SplitSeq(string(data), "\n") {
		if strings.TrimSpace(line) == entry {
			return
		}
	}
	t.Errorf("expected .gitignore to contain %q, got:\n%s", entry, string(data))
}

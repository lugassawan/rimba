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
	assertFileExists(t, filepath.Join(repo, configDir, teamFile))
	assertFileExists(t, filepath.Join(repo, configDir, localFile))

	// Worktree dir is created using convention
	wtDir := filepath.Join(repo, config.DefaultWorktreeDir(filepath.Base(repo)))
	assertFileExists(t, wtDir)

	// .gitignore is created with .rimba/settings.local.toml
	assertFileExists(t, filepath.Join(repo, gitignoreFile))
	localEntry := filepath.Join(configDir, localFile)
	assertGitignoreContains(t, repo, localEntry)
}

func TestInitConfigDefaults(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupRepo(t)
	rimbaSuccess(t, repo, "init")

	// Raw config should only have copy_files (worktree_dir/default_source auto-derived)
	raw, err := config.Resolve(repo)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	if raw.WorktreeDir != "" {
		t.Errorf("expected raw worktree_dir to be empty, got %q", raw.WorktreeDir)
	}
	if raw.DefaultSource != "" {
		t.Errorf("expected raw default_source to be empty, got %q", raw.DefaultSource)
	}
	if len(raw.CopyFiles) == 0 {
		t.Errorf("expected copy_files to be non-empty")
	}

	// After FillDefaults, fields should be populated
	raw.FillDefaults(filepath.Base(repo), branchMain)
	if raw.WorktreeDir == "" {
		t.Errorf("expected worktree_dir to be non-empty after FillDefaults")
	}
	if raw.DefaultSource != branchMain {
		t.Errorf("expected default_source %q, got %q", branchMain, raw.DefaultSource)
	}
}

func TestInitExistingConfigInstallsAgentFiles(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupRepo(t)
	rimbaSuccess(t, repo, "init", "--agents")

	// Re-running init should succeed and update agent files
	r := rimbaSuccess(t, repo, "init", "--agents")
	assertContains(t, r.Stdout, "already exists")
	assertContains(t, r.Stdout, "rimba")
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

	localEntry := filepath.Join(configDir, localFile)
	r := rimbaSuccess(t, repo, "init")
	assertContains(t, r.Stdout, localEntry+" added to .gitignore")
	assertGitignoreContains(t, repo, localEntry)

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
	localEntry := filepath.Join(configDir, localFile)
	// Pre-create .gitignore already containing the entry
	if err := os.WriteFile(filepath.Join(repo, gitignoreFile), []byte(localEntry+"\n"), 0644); err != nil {
		t.Fatalf("failed to write %s: %v", gitignoreFile, err)
	}

	r := rimbaSuccess(t, repo, "init")
	assertContains(t, r.Stdout, "already in .gitignore")

	// Verify no duplicate
	data, err := os.ReadFile(filepath.Join(repo, gitignoreFile))
	if err != nil {
		t.Fatalf("failed to read %s: %v", gitignoreFile, err)
	}
	if strings.Count(string(data), localEntry) != 1 {
		t.Errorf("expected exactly one %s entry, got:\n%s", localEntry, string(data))
	}
}

func TestInitMigratesLegacyConfig(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupRepo(t)

	// Create legacy .rimba.toml
	legacyCfg := config.DefaultConfig(filepath.Base(repo), branchMain)
	if err := config.Save(filepath.Join(repo, configFile), legacyCfg); err != nil {
		t.Fatalf("save legacy config: %v", err)
	}

	// Create .gitignore with legacy entry
	if err := os.WriteFile(filepath.Join(repo, gitignoreFile), []byte(configFile+"\n"), 0644); err != nil {
		t.Fatal(err)
	}

	r := rimbaSuccess(t, repo, "init")
	assertContains(t, r.Stdout, "Migrated rimba config")

	// Verify legacy file is gone
	assertFileNotExists(t, filepath.Join(repo, configFile))

	// Verify new files exist
	assertFileExists(t, filepath.Join(repo, configDir, teamFile))
	assertFileExists(t, filepath.Join(repo, configDir, localFile))

	// Verify config is loadable
	cfg, err := config.Resolve(repo)
	if err != nil {
		t.Fatalf("Resolve after migration: %v", err)
	}
	if cfg.DefaultSource != branchMain {
		t.Errorf("DefaultSource = %q, want %q", cfg.DefaultSource, branchMain)
	}

	// Verify .gitignore updated
	data, err := os.ReadFile(filepath.Join(repo, gitignoreFile))
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)
	if strings.Contains(content, configFile) {
		t.Error(".gitignore should not contain legacy entry after migration")
	}
	localEntry := filepath.Join(configDir, localFile)
	assertGitignoreContains(t, repo, localEntry)
}

func TestInitFailsOutsideGitRepo(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	dir := t.TempDir() // not a git repo
	rimbaFail(t, dir, "init")
}

func TestInitWithAgentsFlag(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupRepo(t)
	r := rimbaSuccess(t, repo, "init", "--agents")

	assertContains(t, r.Stdout, "Installed rimba agent files")
	assertFileExists(t, filepath.Join(repo, "AGENTS.md"))
	assertFileExists(t, filepath.Join(repo, ".github", "copilot-instructions.md"))
	assertFileExists(t, filepath.Join(repo, ".cursor", "rules", "rimba.mdc"))
	assertFileExists(t, filepath.Join(repo, ".claude", "skills", "rimba", "SKILL.md"))
	assertFileExists(t, filepath.Join(repo, "GEMINI.md"))
	assertFileExists(t, filepath.Join(repo, ".windsurf", "rules", "rimba.md"))
	assertFileExists(t, filepath.Join(repo, ".clinerules", "rimba.md"))
}

func TestInitAgentsLocalAddsGitignore(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupRepo(t)
	rimbaSuccess(t, repo, "init", "--agents", "--local")

	// All 7 project spec rel paths should be in .gitignore
	assertGitignoreContains(t, repo, "AGENTS.md")
	assertGitignoreContains(t, repo, "GEMINI.md")
	assertGitignoreContains(t, repo, filepath.Join(".windsurf", "rules", "rimba.md"))
}

func TestInitAgentsUninstall(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupRepo(t)
	rimbaSuccess(t, repo, "init", "--agents")
	r := rimbaSuccess(t, repo, "init", "--agents", "--uninstall")

	assertContains(t, r.Stdout, "Removed rimba agent files")
	assertFileNotExists(t, filepath.Join(repo, ".cursor", "rules", "rimba.mdc"))
}

func TestInitGlobalOutsideRepo(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	home := t.TempDir()
	t.Setenv("HOME", home)

	// Run from a non-git directory
	dir := t.TempDir()
	r := rimbaSuccess(t, dir, "init", "-g")

	assertContains(t, r.Stdout, "Installed rimba agent files (user)")
	assertFileExists(t, filepath.Join(home, ".claude", "skills", "rimba", "SKILL.md"))
	assertFileExists(t, filepath.Join(home, ".cursor", "rules", "rimba.mdc"))
	assertFileExists(t, filepath.Join(home, ".codex", "AGENTS.md"))
	assertFileExists(t, filepath.Join(home, ".gemini", "GEMINI.md"))
	assertFileExists(t, filepath.Join(home, ".codeium", "windsurf", "memories", "global_rules.md"))
	assertFileExists(t, filepath.Join(home, ".roo", "rules", "rimba.md"))
}

func TestInitGlobalUninstall(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	home := t.TempDir()
	t.Setenv("HOME", home)

	dir := t.TempDir()
	rimbaSuccess(t, dir, "init", "-g")
	r := rimbaSuccess(t, dir, "init", "-g", "--uninstall")

	assertContains(t, r.Stdout, "Removed rimba agent files (user)")
	assertFileNotExists(t, filepath.Join(home, ".claude", "skills", "rimba", "SKILL.md"))
}

func TestInitOldAgentFilesFlagFails(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupRepo(t)
	rimbaFail(t, repo, "init", "--agent-files")
}

func TestInitSkipsAgentFilesWithoutFlag(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupRepo(t)
	r := rimbaSuccess(t, repo, "init")

	assertNotContains(t, r.Stdout, "rimba agent files")
	assertFileNotExists(t, filepath.Join(repo, "AGENTS.md"))
}

func TestInitPersonalFreshInit(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupRepo(t)
	r := rimbaSuccess(t, repo, "init", "--personal")

	assertContains(t, r.Stdout, "Initialized rimba")
	assertFileExists(t, filepath.Join(repo, configDir, teamFile))
	assertFileNotExists(t, filepath.Join(repo, configDir, localFile))

	// .gitignore has .rimba/ not .rimba/settings.local.toml
	dirEntry := configDir + "/"
	assertGitignoreContains(t, repo, dirEntry)
	localEntry := filepath.Join(configDir, localFile)
	assertGitignoreNotContains(t, repo, localEntry)
}

func TestInitPersonalMigration(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupRepo(t)

	// Create legacy .rimba.toml
	legacyCfg := config.DefaultConfig(filepath.Base(repo), branchMain)
	if err := config.Save(filepath.Join(repo, configFile), legacyCfg); err != nil {
		t.Fatalf("save legacy config: %v", err)
	}

	// Create .gitignore with legacy entry
	if err := os.WriteFile(filepath.Join(repo, gitignoreFile), []byte(configFile+"\n"), 0644); err != nil {
		t.Fatal(err)
	}

	r := rimbaSuccess(t, repo, "init", "--personal")
	assertContains(t, r.Stdout, "Migrated rimba config")

	// Verify legacy file is gone
	assertFileNotExists(t, filepath.Join(repo, configFile))

	// Verify new config exists and is loadable
	assertFileExists(t, filepath.Join(repo, configDir, teamFile))
	assertFileNotExists(t, filepath.Join(repo, configDir, localFile))

	cfg, err := config.Resolve(repo)
	if err != nil {
		t.Fatalf("Resolve after migration: %v", err)
	}
	if cfg.DefaultSource != branchMain {
		t.Errorf("DefaultSource = %q, want %q", cfg.DefaultSource, branchMain)
	}

	// Verify .gitignore updated with .rimba/ not legacy entry
	data, err := os.ReadFile(filepath.Join(repo, gitignoreFile))
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)
	if strings.Contains(content, configFile) {
		t.Error(".gitignore should not contain legacy entry after migration")
	}
	dirEntry := configDir + "/"
	assertGitignoreContains(t, repo, dirEntry)
	localEntry := filepath.Join(configDir, localFile)
	assertGitignoreNotContains(t, repo, localEntry)
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

// assertGitignoreNotContains verifies that .gitignore in the repo does NOT contain the given entry.
func assertGitignoreNotContains(t *testing.T, repo, entry string) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(repo, gitignoreFile))
	if err != nil {
		t.Fatalf("failed to read .gitignore: %v", err)
	}
	for line := range strings.SplitSeq(string(data), "\n") {
		if strings.TrimSpace(line) == entry {
			t.Errorf("expected .gitignore NOT to contain %q, got:\n%s", entry, string(data))
			return
		}
	}
}

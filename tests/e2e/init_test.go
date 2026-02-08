package e2e_test

import (
	"path/filepath"
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

func TestInitFailsIfAlreadyInitialized(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupRepo(t)
	rimbaSuccess(t, repo, "init")

	r := rimbaFail(t, repo, "init")
	assertContains(t, r.Stderr, "already exists")
}

func TestInitFailsOutsideGitRepo(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	dir := t.TempDir() // not a git repo
	rimbaFail(t, dir, "init")
}

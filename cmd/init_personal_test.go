package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lugassawan/rimba/internal/config"
)

func TestInitPersonalFreshInit(t *testing.T) {
	repoDir := t.TempDir()

	r := repoRootRunner(repoDir, func(args ...string) (string, error) {
		if args[0] == cmdSymbolicRef {
			return refsRemotesOriginMain, nil
		}
		return "", nil
	})
	restore := overrideNewRunner(r)
	defer restore()

	cmd, buf := newTestCmd()
	cmd.Flags().Bool(flagPersonal, true, "")

	if err := initCmd.RunE(cmd, nil); err != nil {
		t.Fatalf("initCmd.RunE: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "Initialized rimba") {
		t.Errorf("output = %q, want 'Initialized rimba'", out)
	}

	// Verify .rimba/settings.toml was created
	teamPath := filepath.Join(repoDir, config.DirName, config.TeamFile)
	if _, err := os.Stat(teamPath); os.IsNotExist(err) {
		t.Errorf("settings.toml not created at %s", teamPath)
	}

	// Verify .rimba/settings.local.toml was NOT created
	localPath := filepath.Join(repoDir, config.DirName, config.LocalFile)
	if _, err := os.Stat(localPath); err == nil {
		t.Error("settings.local.toml should not be created in personal mode")
	}

	// Verify .gitignore has .rimba/ not .rimba/settings.local.toml
	data, err := os.ReadFile(filepath.Join(repoDir, ".gitignore"))
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)
	dirEntry := config.DirName + "/"
	if !strings.Contains(content, dirEntry) {
		t.Errorf(".gitignore should contain %q, got:\n%s", dirEntry, content)
	}
	localEntry := filepath.Join(config.DirName, config.LocalFile)
	if strings.Contains(content, localEntry) {
		t.Errorf(".gitignore should not contain %q in personal mode, got:\n%s", localEntry, content)
	}
}

func TestInitPersonalMigration(t *testing.T) {
	repoDir := t.TempDir()

	// Pre-create legacy .rimba.toml
	legacyCfg := &config.Config{WorktreeDir: "../wt", DefaultSource: "main"}
	if err := config.Save(filepath.Join(repoDir, config.FileName), legacyCfg); err != nil {
		t.Fatalf("save legacy config: %v", err)
	}

	// Pre-create .gitignore with legacy entry
	if err := os.WriteFile(filepath.Join(repoDir, ".gitignore"), []byte(config.FileName+"\n"), 0644); err != nil {
		t.Fatal(err)
	}

	r := repoRootRunner(repoDir, func(_ ...string) (string, error) { return "", nil })
	restore := overrideNewRunner(r)
	defer restore()

	cmd, buf := newTestCmd()
	cmd.Flags().Bool(flagPersonal, true, "")

	if err := initCmd.RunE(cmd, nil); err != nil {
		t.Fatalf("initCmd.RunE: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "Migrated rimba config") {
		t.Errorf("output should contain 'Migrated rimba config', got:\n%s", out)
	}

	// Verify legacy file is gone
	if _, err := os.Stat(filepath.Join(repoDir, config.FileName)); !os.IsNotExist(err) {
		t.Error("legacy .rimba.toml should have been removed")
	}

	// Verify .rimba/settings.toml exists and is loadable
	cfg, err := config.LoadDir(filepath.Join(repoDir, config.DirName))
	if err != nil {
		t.Fatalf("LoadDir after migration: %v", err)
	}
	if cfg.WorktreeDir != "../wt" {
		t.Errorf("WorktreeDir = %q, want %q", cfg.WorktreeDir, "../wt")
	}

	// Verify .rimba/settings.local.toml was NOT created
	localPath := filepath.Join(repoDir, config.DirName, config.LocalFile)
	if _, err := os.Stat(localPath); err == nil {
		t.Error("settings.local.toml should not be created in personal mode")
	}

	// Verify .gitignore updated with .rimba/ not legacy entry
	data, err := os.ReadFile(filepath.Join(repoDir, ".gitignore"))
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)
	if strings.Contains(content, config.FileName) {
		t.Error(".gitignore should not contain legacy entry after migration")
	}
	dirEntry := config.DirName + "/"
	if !strings.Contains(content, dirEntry) {
		t.Errorf(".gitignore should contain %q, got:\n%s", dirEntry, content)
	}
	localEntry := filepath.Join(config.DirName, config.LocalFile)
	if strings.Contains(content, localEntry) {
		t.Errorf(".gitignore should not contain %q in personal mode, got:\n%s", localEntry, content)
	}
}

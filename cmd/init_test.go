package cmd

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lugassawan/rimba/internal/config"
)

func TestInitSuccess(t *testing.T) {
	repoDir := t.TempDir()

	r := &mockRunner{
		run: func(args ...string) (string, error) {
			if len(args) >= 2 && args[1] == cmdShowToplevel {
				return repoDir, nil
			}
			if len(args) >= 1 && args[0] == cmdSymbolicRef {
				return refsRemotesOriginMain, nil
			}
			return "", nil
		},
		runInDir: noopRunInDir,
	}
	restore := overrideNewRunner(r)
	defer restore()

	cmd, buf := newTestCmd()

	err := initCmd.RunE(cmd, nil)
	if err != nil {
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

	// Verify .rimba/settings.local.toml was created
	localPath := filepath.Join(repoDir, config.DirName, config.LocalFile)
	if _, err := os.Stat(localPath); os.IsNotExist(err) {
		t.Errorf("settings.local.toml not created at %s", localPath)
	}

	// Verify worktree dir was created
	repoName := filepath.Base(repoDir)
	wtDir := filepath.Join(repoDir, config.DefaultWorktreeDir(repoName))
	if _, err := os.Stat(wtDir); os.IsNotExist(err) {
		t.Errorf("worktree dir not created at %s", wtDir)
	}
}

func TestInitMigrationFromLegacy(t *testing.T) {
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

	r := &mockRunner{
		run: func(args ...string) (string, error) {
			if len(args) >= 2 && args[1] == cmdShowToplevel {
				return repoDir, nil
			}
			return "", nil
		},
		runInDir: noopRunInDir,
	}
	restore := overrideNewRunner(r)
	defer restore()

	cmd, buf := newTestCmd()

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

	// Verify .rimba/settings.local.toml exists
	localPath := filepath.Join(repoDir, config.DirName, config.LocalFile)
	if _, err := os.Stat(localPath); os.IsNotExist(err) {
		t.Error("settings.local.toml should have been created")
	}

	// Verify .gitignore updated
	data, err := os.ReadFile(filepath.Join(repoDir, ".gitignore"))
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)
	if strings.Contains(content, config.FileName) {
		t.Error(".gitignore should not contain legacy entry after migration")
	}
	localEntry := filepath.Join(config.DirName, config.LocalFile)
	if !strings.Contains(content, localEntry) {
		t.Errorf(".gitignore should contain %q, got:\n%s", localEntry, content)
	}
}

func TestInitExistingDirConfig(t *testing.T) {
	repoDir := t.TempDir()

	// Create .rimba/ directory
	rimbaDir := filepath.Join(repoDir, config.DirName)
	if err := os.MkdirAll(rimbaDir, 0755); err != nil {
		t.Fatal(err)
	}

	r := &mockRunner{
		run: func(args ...string) (string, error) {
			if len(args) >= 2 && args[1] == cmdShowToplevel {
				return repoDir, nil
			}
			return "", nil
		},
		runInDir: noopRunInDir,
	}
	restore := overrideNewRunner(r)
	defer restore()

	cmd, buf := newTestCmd()

	if err := initCmd.RunE(cmd, nil); err != nil {
		t.Fatalf("initCmd.RunE: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "already exists") {
		t.Errorf("output should contain 'already exists', got:\n%s", out)
	}
}

func TestInitCreatesAgentFiles(t *testing.T) {
	repoDir := t.TempDir()

	r := &mockRunner{
		run: func(args ...string) (string, error) {
			if len(args) >= 2 && args[1] == cmdShowToplevel {
				return repoDir, nil
			}
			if len(args) >= 1 && args[0] == cmdSymbolicRef {
				return refsRemotesOriginMain, nil
			}
			return "", nil
		},
		runInDir: noopRunInDir,
	}
	restore := overrideNewRunner(r)
	defer restore()

	cmd, buf := newTestCmd()
	cmd.Flags().Bool(flagAgentFiles, true, "")

	if err := initCmd.RunE(cmd, nil); err != nil {
		t.Fatalf("initCmd.RunE: %v", err)
	}

	out := buf.String()

	// Verify agent file output lines
	agentFiles := []string{
		"AGENTS.md",
		filepath.Join(".github", "copilot-instructions.md"),
		filepath.Join(".cursor", "rules", "rimba.mdc"),
		filepath.Join(".claude", "skills", "rimba", "SKILL.md"),
	}

	for _, f := range agentFiles {
		if !strings.Contains(out, f) {
			t.Errorf("output should mention %q", f)
		}
		path := filepath.Join(repoDir, f)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Errorf("agent file not created: %s", f)
		}
	}
}

func TestInitExistingConfigInstallsAgentFiles(t *testing.T) {
	repoDir := t.TempDir()

	// Create .rimba/ directory to simulate existing config
	rimbaDir := filepath.Join(repoDir, config.DirName)
	if err := os.MkdirAll(rimbaDir, 0755); err != nil {
		t.Fatal(err)
	}

	r := &mockRunner{
		run: func(args ...string) (string, error) {
			if len(args) >= 2 && args[1] == cmdShowToplevel {
				return repoDir, nil
			}
			if len(args) >= 1 && args[0] == cmdSymbolicRef {
				return refsRemotesOriginMain, nil
			}
			return "", nil
		},
		runInDir: noopRunInDir,
	}
	restore := overrideNewRunner(r)
	defer restore()

	cmd, buf := newTestCmd()
	cmd.Flags().Bool(flagAgentFiles, true, "")

	// Should succeed (not error) when .rimba/ already exists
	err := initCmd.RunE(cmd, nil)
	if err != nil {
		t.Fatalf("initCmd.RunE should succeed with existing config, got: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "already exists") {
		t.Error("output should note config already exists")
	}

	// Agent files should still be created
	if _, err := os.Stat(filepath.Join(repoDir, "AGENTS.md")); os.IsNotExist(err) {
		t.Error("AGENTS.md should be created even when config exists")
	}
}

func TestInitAgentFilesIdempotent(t *testing.T) {
	repoDir := t.TempDir()

	r := &mockRunner{
		run: func(args ...string) (string, error) {
			if len(args) >= 2 && args[1] == cmdShowToplevel {
				return repoDir, nil
			}
			if len(args) >= 1 && args[0] == cmdSymbolicRef {
				return refsRemotesOriginMain, nil
			}
			return "", nil
		},
		runInDir: noopRunInDir,
	}
	restore := overrideNewRunner(r)
	defer restore()

	// First init
	cmd1, _ := newTestCmd()
	cmd1.Flags().Bool(flagAgentFiles, true, "")
	if err := initCmd.RunE(cmd1, nil); err != nil {
		t.Fatalf("first initCmd.RunE: %v", err)
	}

	// Second init (config exists now)
	cmd2, buf2 := newTestCmd()
	cmd2.Flags().Bool(flagAgentFiles, true, "")
	if err := initCmd.RunE(cmd2, nil); err != nil {
		t.Fatalf("second initCmd.RunE: %v", err)
	}

	out := buf2.String()
	if !strings.Contains(out, "already exists") {
		t.Error("second init should note config already exists")
	}

	// AGENTS.md should not have duplicated markers
	content, err := os.ReadFile(filepath.Join(repoDir, "AGENTS.md"))
	if err != nil {
		t.Fatalf("read AGENTS.md: %v", err)
	}
	if strings.Count(string(content), "<!-- BEGIN RIMBA -->") != 1 {
		t.Error("AGENTS.md should have exactly one BEGIN RIMBA marker after re-init")
	}
}

func TestInitWithoutFlagSkipsAgentFiles(t *testing.T) {
	repoDir := t.TempDir()

	r := &mockRunner{
		run: func(args ...string) (string, error) {
			if len(args) >= 2 && args[1] == cmdShowToplevel {
				return repoDir, nil
			}
			if len(args) >= 1 && args[0] == cmdSymbolicRef {
				return refsRemotesOriginMain, nil
			}
			return "", nil
		},
		runInDir: noopRunInDir,
	}
	restore := overrideNewRunner(r)
	defer restore()

	cmd, buf := newTestCmd()

	if err := initCmd.RunE(cmd, nil); err != nil {
		t.Fatalf("initCmd.RunE: %v", err)
	}

	out := buf.String()
	if strings.Contains(out, "Agent:") {
		t.Errorf("output should not mention agent files without --agent-files flag, got:\n%s", out)
	}

	if _, err := os.Stat(filepath.Join(repoDir, "AGENTS.md")); err == nil {
		t.Error("AGENTS.md should not be created without --agent-files flag")
	}
}

func TestInitPersonalFreshInit(t *testing.T) {
	repoDir := t.TempDir()

	r := &mockRunner{
		run: func(args ...string) (string, error) {
			if len(args) >= 2 && args[1] == cmdShowToplevel {
				return repoDir, nil
			}
			if len(args) >= 1 && args[0] == cmdSymbolicRef {
				return refsRemotesOriginMain, nil
			}
			return "", nil
		},
		runInDir: noopRunInDir,
	}
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

	r := &mockRunner{
		run: func(args ...string) (string, error) {
			if len(args) >= 2 && args[1] == cmdShowToplevel {
				return repoDir, nil
			}
			return "", nil
		},
		runInDir: noopRunInDir,
	}
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

func TestInitRepoRootError(t *testing.T) {
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			if len(args) >= 2 && args[1] == cmdShowToplevel {
				return "", errors.New("not a git repository")
			}
			return "", nil
		},
		runInDir: noopRunInDir,
	}
	restore := overrideNewRunner(r)
	defer restore()

	cmd, _ := newTestCmd()

	err := initCmd.RunE(cmd, nil)
	if err == nil {
		t.Fatal("expected error when repo root fails")
	}
}

package cmd

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lugassawan/rimba/internal/config"
	"github.com/spf13/cobra"
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
	cmd.Flags().Bool(flagAgents, true, "")

	if err := initCmd.RunE(cmd, nil); err != nil {
		t.Fatalf("initCmd.RunE: %v", err)
	}

	out := buf.String()

	// Verify agent file output lines for all 7 project specs
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
	cmd.Flags().Bool(flagAgents, true, "")

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
	cmd1.Flags().Bool(flagAgents, true, "")
	if err := initCmd.RunE(cmd1, nil); err != nil {
		t.Fatalf("first initCmd.RunE: %v", err)
	}

	// Second init (config exists now)
	cmd2, buf2 := newTestCmd()
	cmd2.Flags().Bool(flagAgents, true, "")
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
		t.Errorf("output should not mention agent files without agent flag, got:\n%s", out)
	}

	if _, err := os.Stat(filepath.Join(repoDir, "AGENTS.md")); err == nil {
		t.Error("AGENTS.md should not be created without --agents flag")
	}
}

func TestInitAgentsErrors(t *testing.T) {
	tests := []struct {
		name        string
		blockPath   string // relative path to pre-create as a directory to trigger the error
		wantErrFrag string // substring expected in the error
	}{
		{
			name:        "install error (AGENTS.md is a directory)",
			blockPath:   "AGENTS.md",
			wantErrFrag: "agent files",
		},
		{
			// Pre-create .mcp.json as a directory so os.ReadFile returns EISDIR (not ErrNotExist),
			// causing RegisterMCPProject to fail after InstallProject succeeds.
			name:        "MCP register error (.mcp.json is a directory)",
			blockPath:   ".mcp.json",
			wantErrFrag: "mcp servers",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repoDir := t.TempDir()
			if err := os.MkdirAll(filepath.Join(repoDir, tt.blockPath), 0750); err != nil {
				t.Fatal(err)
			}

			repoName := filepath.Base(repoDir)
			wtPath := filepath.Join(filepath.Dir(repoDir), repoName+"-worktrees")
			t.Cleanup(func() { os.RemoveAll(wtPath) })

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

			cmd, _ := newTestCmd()
			cmd.Flags().Bool(flagAgents, true, "")
			err := initCmd.RunE(cmd, nil)
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tt.wantErrFrag)
			}
			if !strings.Contains(err.Error(), tt.wantErrFrag) {
				t.Errorf("error = %q, want %q", err.Error(), tt.wantErrFrag)
			}
		})
	}
}

func TestInitMigrateGitignoreWriteError(t *testing.T) {
	repoDir := t.TempDir()

	// Pre-create legacy .rimba.toml so migration case triggers.
	if err := os.WriteFile(filepath.Join(repoDir, config.FileName), []byte("[rimba]\n"), 0600); err != nil {
		t.Fatal(err)
	}
	// Read-only .gitignore: RemoveGitignoreEntry fails silently; EnsureGitignore fails loudly.
	gitignorePath := filepath.Join(repoDir, ".gitignore")
	if err := os.WriteFile(gitignorePath, []byte("# existing\n"), 0444); err != nil {
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

	cmd, _ := newTestCmd()
	err := initCmd.RunE(cmd, nil)
	if err == nil {
		t.Fatal("expected error when .gitignore is read-only during migration, got nil")
	}
	if !strings.Contains(err.Error(), "failed to update .gitignore") {
		t.Errorf("error = %q, want 'failed to update .gitignore'", err.Error())
	}
}

func TestInitFreshWorktreeDirConflict(t *testing.T) {
	repoDir := t.TempDir()

	// Pre-create a FILE at the worktree path so os.MkdirAll(wtDir) fails.
	// wtDir = filepath.Join(repoDir, "../repoName-worktrees")
	repoName := filepath.Base(repoDir)
	wtPath := filepath.Join(filepath.Dir(repoDir), repoName+"-worktrees")
	if err := os.WriteFile(wtPath, nil, 0600); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Remove(wtPath) })

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

	cmd, _ := newTestCmd()
	err := initCmd.RunE(cmd, nil)
	if err == nil {
		t.Fatal("expected error when worktree dir is a file, got nil")
	}
	if !strings.Contains(err.Error(), "failed to create worktree directory") {
		t.Errorf("error = %q, want 'failed to create worktree directory'", err.Error())
	}
}

func TestInitFreshGitignoreWriteError(t *testing.T) {
	repoDir := t.TempDir()

	// Remove the worktree dir created outside repoDir (../repoName-worktrees).
	repoName := filepath.Base(repoDir)
	wtDir := filepath.Join(repoDir, "..", repoName+"-worktrees")
	t.Cleanup(func() { os.RemoveAll(wtDir) })

	// Pre-create .gitignore as read-only without the rimba entry.
	gitignorePath := filepath.Join(repoDir, ".gitignore")
	if err := os.WriteFile(gitignorePath, []byte("# existing\n"), 0444); err != nil {
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

	cmd, _ := newTestCmd()
	err := initCmd.RunE(cmd, nil)
	if err == nil {
		t.Fatal("expected error when .gitignore is read-only, got nil")
	}
	if !strings.Contains(err.Error(), "failed to update .gitignore") {
		t.Errorf("error = %q, want 'failed to update .gitignore'", err.Error())
	}
}

func TestInitFreshConflictingFile(t *testing.T) {
	repoDir := t.TempDir()

	// Pre-create a file at dirPath so os.MkdirAll fails in runInitFresh.
	dirPath := filepath.Join(repoDir, config.DirName)
	if err := os.WriteFile(dirPath, nil, 0600); err != nil {
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

	cmd, _ := newTestCmd()
	err := initCmd.RunE(cmd, nil)
	if err == nil {
		t.Fatal("expected error when dirPath is a file, got nil")
	}
	if !strings.Contains(err.Error(), "failed to create config directory") {
		t.Errorf("error = %q, want 'failed to create config directory'", err.Error())
	}
}

func TestInitFreshRepoNameError(t *testing.T) {
	repoDir := t.TempDir()

	// First --show-toplevel call succeeds (RepoRoot in RunE); second fails (RepoName in runInitFresh).
	callCount := 0
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			if len(args) >= 2 && args[1] == cmdShowToplevel {
				callCount++
				if callCount == 1 {
					return repoDir, nil
				}
				return "", errors.New("repo name lookup failed")
			}
			return "", nil
		},
		runInDir: noopRunInDir,
	}
	restore := overrideNewRunner(r)
	defer restore()

	cmd, _ := newTestCmd()
	if err := initCmd.RunE(cmd, nil); err == nil {
		t.Fatal("expected error when RepoName fails, got nil")
	}
}

func TestInitFreshDefaultBranchError(t *testing.T) {
	repoDir := t.TempDir()

	// Return repo root for --show-toplevel; fail everything else so DefaultBranch errors.
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			if len(args) >= 2 && args[1] == cmdShowToplevel {
				return repoDir, nil
			}
			return "", errors.New("command not found")
		},
		runInDir: noopRunInDir,
	}
	restore := overrideNewRunner(r)
	defer restore()

	cmd, _ := newTestCmd()
	err := initCmd.RunE(cmd, nil)
	if err == nil {
		t.Fatal("expected error when DefaultBranch fails, got nil")
	}
}

func TestInitMigrateConflictingDir(t *testing.T) {
	repoDir := t.TempDir()

	// Pre-create legacy .rimba.toml so migration case triggers.
	legacyPath := filepath.Join(repoDir, config.FileName)
	if err := os.WriteFile(legacyPath, nil, 0600); err != nil {
		t.Fatal(err)
	}
	// Pre-create a FILE at dirPath so os.MkdirAll fails.
	dirPath := filepath.Join(repoDir, config.DirName)
	if err := os.WriteFile(dirPath, nil, 0600); err != nil {
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

	cmd, _ := newTestCmd()
	err := initCmd.RunE(cmd, nil)
	if err == nil {
		t.Fatal("expected error when dirPath is a file, got nil")
	}
	if !strings.Contains(err.Error(), "failed to create config directory") {
		t.Errorf("error = %q, want to contain 'failed to create config directory'", err.Error())
	}
}

func TestInitFreshGitignoreAlreadyPresent(t *testing.T) {
	repoDir := t.TempDir()

	// Pre-seed .gitignore with the entry that rimba would add
	gitignoreEntry := filepath.Join(config.DirName, config.LocalFile)
	if err := os.WriteFile(filepath.Join(repoDir, ".gitignore"), []byte(gitignoreEntry+"\n"), 0644); err != nil {
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
	if err := initCmd.RunE(cmd, nil); err != nil {
		t.Fatalf("initCmd.RunE: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "already in .gitignore") {
		t.Errorf("output should say 'already in .gitignore', got:\n%s", out)
	}
}

// --- Tests for new 3-tier flag surface ---

func TestInitGlobalInstall(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	// -g does not need a git repo
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			return "", errors.New("not a git repository")
		},
		runInDir: noopRunInDir,
	}
	restore := overrideNewRunner(r)
	defer restore()

	cmd, buf := newTestCmd()
	cmd.Flags().Bool(flagGlobal, true, "")

	if err := initCmd.RunE(cmd, nil); err != nil {
		t.Fatalf("initCmd.RunE: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "user") {
		t.Errorf("output should mention 'user', got:\n%s", out)
	}

	// Verify Claude skill was created
	claudePath := filepath.Join(home, ".claude", "skills", "rimba", "SKILL.md")
	if _, err := os.Stat(claudePath); os.IsNotExist(err) {
		t.Errorf("global claude skill not created at %s", claudePath)
	}
}

func TestInitGlobalNoRepoRequired(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	// Simulate git.RepoRoot failing (not in a repo)
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			return "", errors.New("not a git repository")
		},
		runInDir: noopRunInDir,
	}
	restore := overrideNewRunner(r)
	defer restore()

	cmd, _ := newTestCmd()
	cmd.Flags().Bool(flagGlobal, true, "")

	if err := initCmd.RunE(cmd, nil); err != nil {
		t.Fatalf("-g should succeed outside a git repo, got: %v", err)
	}
}

func TestInitGlobalUninstall(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	// Install first
	r := &mockRunner{
		run:      func(...string) (string, error) { return "", errors.New("not a git repo") },
		runInDir: noopRunInDir,
	}
	restore := overrideNewRunner(r)
	defer restore()

	cmd1, _ := newTestCmd()
	cmd1.Flags().Bool(flagGlobal, true, "")
	if err := initCmd.RunE(cmd1, nil); err != nil {
		t.Fatalf("install: %v", err)
	}

	// Uninstall
	cmd2, buf := newTestCmd()
	cmd2.Flags().Bool(flagGlobal, true, "")
	cmd2.Flags().Bool(flagUninstall, true, "")
	if err := initCmd.RunE(cmd2, nil); err != nil {
		t.Fatalf("uninstall: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "Removed") {
		t.Errorf("output should say 'Removed', got:\n%s", out)
	}

	// Claude skill should be gone
	claudePath := filepath.Join(home, ".claude", "skills", "rimba", "SKILL.md")
	if _, err := os.Stat(claudePath); err == nil {
		t.Error("claude skill should be removed after uninstall")
	}
}

func TestInitAgentsLocalAddsGitignore(t *testing.T) {
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

	cmd, _ := newTestCmd()
	cmd.Flags().Bool(flagAgents, true, "")
	cmd.Flags().Bool(flagLocal, true, "")

	if err := initCmd.RunE(cmd, nil); err != nil {
		t.Fatalf("initCmd.RunE: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(repoDir, ".gitignore"))
	if err != nil {
		t.Fatalf("read .gitignore: %v", err)
	}
	gitignore := string(data)
	// AGENTS.md should be in .gitignore (local tier)
	if !strings.Contains(gitignore, "AGENTS.md") {
		t.Errorf(".gitignore should contain AGENTS.md, got:\n%s", gitignore)
	}
}

func TestInitAgentsUninstall(t *testing.T) {
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

	// Install
	cmd1, _ := newTestCmd()
	cmd1.Flags().Bool(flagAgents, true, "")
	if err := initCmd.RunE(cmd1, nil); err != nil {
		t.Fatalf("install: %v", err)
	}
	agentsPath := filepath.Join(repoDir, "AGENTS.md")
	if _, err := os.Stat(agentsPath); os.IsNotExist(err) {
		t.Fatal("AGENTS.md should exist after install")
	}

	// Uninstall
	cmd2, buf := newTestCmd()
	cmd2.Flags().Bool(flagAgents, true, "")
	cmd2.Flags().Bool(flagUninstall, true, "")
	if err := initCmd.RunE(cmd2, nil); err != nil {
		t.Fatalf("uninstall: %v", err)
	}
	if !strings.Contains(buf.String(), "Removed") {
		t.Errorf("output should say 'Removed', got:\n%s", buf.String())
	}
	if _, err := os.Stat(agentsPath); err == nil {
		t.Error("AGENTS.md should be removed after uninstall")
	}
}

func TestInitAgentsLocalUninstall(t *testing.T) {
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

	// Install local
	cmd1, _ := newTestCmd()
	cmd1.Flags().Bool(flagAgents, true, "")
	cmd1.Flags().Bool(flagLocal, true, "")
	if err := initCmd.RunE(cmd1, nil); err != nil {
		t.Fatalf("install: %v", err)
	}
	agentsPath := filepath.Join(repoDir, "AGENTS.md")
	if _, err := os.Stat(agentsPath); os.IsNotExist(err) {
		t.Fatal("AGENTS.md should exist after local install")
	}

	// Uninstall local
	cmd2, buf := newTestCmd()
	cmd2.Flags().Bool(flagAgents, true, "")
	cmd2.Flags().Bool(flagLocal, true, "")
	cmd2.Flags().Bool(flagUninstall, true, "")
	if err := initCmd.RunE(cmd2, nil); err != nil {
		t.Fatalf("uninstall: %v", err)
	}
	if !strings.Contains(buf.String(), "Removed") {
		t.Errorf("output should say 'Removed', got:\n%s", buf.String())
	}
	if _, err := os.Stat(agentsPath); err == nil {
		t.Error("AGENTS.md should be removed after local uninstall")
	}
}

func TestInitFlagValidationErrors(t *testing.T) {
	repoDir := t.TempDir()

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

	tests := []struct {
		name    string
		setup   func(*cobra.Command)
		wantErr string
	}{
		{
			name: "--local without --agents",
			setup: func(cmd *cobra.Command) {
				cmd.Flags().Bool(flagLocal, true, "")
			},
			wantErr: "--local requires --agents",
		},
		{
			name: "--uninstall without target",
			setup: func(cmd *cobra.Command) {
				cmd.Flags().Bool(flagUninstall, true, "")
			},
			wantErr: "--uninstall requires",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd, _ := newTestCmd()
			tt.setup(cmd)
			err := initCmd.RunE(cmd, nil)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("error = %q, want to contain %q", err.Error(), tt.wantErr)
			}
		})
	}
}

// --- MCP registration tests ---

func TestInitGlobalRegistersMCP(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	// Pre-seed .claude/settings.json so MCP registration doesn't skip it
	claudeDir := filepath.Join(home, ".claude")
	if err := os.MkdirAll(claudeDir, 0750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(claudeDir, "settings.json"), []byte("{}\n"), 0600); err != nil {
		t.Fatalf("write settings.json: %v", err)
	}

	r := &mockRunner{
		run:      func(...string) (string, error) { return "", errors.New("not a git repo") },
		runInDir: noopRunInDir,
	}
	restore := overrideNewRunner(r)
	defer restore()

	cmd, buf := newTestCmd()
	cmd.Flags().Bool(flagGlobal, true, "")
	if err := initCmd.RunE(cmd, nil); err != nil {
		t.Fatalf("initCmd.RunE: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "Agent files:") {
		t.Errorf("output should contain 'Agent files:', got:\n%s", out)
	}
	if !strings.Contains(out, "MCP server:") {
		t.Errorf("output should contain 'MCP server:', got:\n%s", out)
	}
	if !strings.Contains(out, filepath.Join(".claude", "settings.json")) {
		t.Errorf("output should mention settings.json, got:\n%s", out)
	}
	if !strings.Contains(out, "registered") {
		t.Errorf("output should say 'registered', got:\n%s", out)
	}

	// Verify the file was patched
	data, err := os.ReadFile(filepath.Join(claudeDir, "settings.json"))
	if err != nil {
		t.Fatalf("read settings.json: %v", err)
	}
	var cfg map[string]any
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("parse settings.json: %v", err)
	}
	servers, ok := cfg["mcpServers"].(map[string]any)
	if !ok {
		t.Fatal("mcpServers missing in settings.json")
	}
	entry, ok := servers["rimba"].(map[string]any)
	if !ok {
		t.Fatal("mcpServers.rimba missing")
	}
	if entry["command"] != "rimba" {
		t.Errorf("command = %v, want rimba", entry["command"])
	}
}

func TestInitGlobalMCPSkippedWhenNoConfig(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	r := &mockRunner{
		run:      func(...string) (string, error) { return "", errors.New("not a git repo") },
		runInDir: noopRunInDir,
	}
	restore := overrideNewRunner(r)
	defer restore()

	cmd, buf := newTestCmd()
	cmd.Flags().Bool(flagGlobal, true, "")
	if err := initCmd.RunE(cmd, nil); err != nil {
		t.Fatalf("initCmd.RunE: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "skipped") {
		t.Errorf("output should contain 'skipped' for absent config files, got:\n%s", out)
	}
}

func TestInitGlobalUninstallRemovesMCP(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	claudeDir := filepath.Join(home, ".claude")
	if err := os.MkdirAll(claudeDir, 0750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(claudeDir, "settings.json"), []byte("{}\n"), 0600); err != nil {
		t.Fatalf("write settings.json: %v", err)
	}

	r := &mockRunner{
		run:      func(...string) (string, error) { return "", errors.New("not a git repo") },
		runInDir: noopRunInDir,
	}
	restore := overrideNewRunner(r)
	defer restore()

	// Install
	cmd1, _ := newTestCmd()
	cmd1.Flags().Bool(flagGlobal, true, "")
	if err := initCmd.RunE(cmd1, nil); err != nil {
		t.Fatalf("install: %v", err)
	}

	// Uninstall
	cmd2, buf := newTestCmd()
	cmd2.Flags().Bool(flagGlobal, true, "")
	cmd2.Flags().Bool(flagUninstall, true, "")
	if err := initCmd.RunE(cmd2, nil); err != nil {
		t.Fatalf("uninstall: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "unregistered") {
		t.Errorf("output should say 'unregistered', got:\n%s", out)
	}

	// rimba key should be gone
	data, err := os.ReadFile(filepath.Join(claudeDir, "settings.json"))
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	var cfg map[string]any
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if servers, ok := cfg["mcpServers"].(map[string]any); ok {
		if _, found := servers["rimba"]; found {
			t.Error("rimba key should be removed after uninstall")
		}
	}
}

func TestInitAgentsMCPRegistration(t *testing.T) {
	repoDir := t.TempDir()

	// Pre-seed .mcp.json so registration doesn't skip
	if err := os.WriteFile(filepath.Join(repoDir, ".mcp.json"), []byte("{}\n"), 0600); err != nil {
		t.Fatalf("write .mcp.json: %v", err)
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
	cmd.Flags().Bool(flagAgents, true, "")
	if err := initCmd.RunE(cmd, nil); err != nil {
		t.Fatalf("initCmd.RunE: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "MCP server:") {
		t.Errorf("output should contain 'MCP server:', got:\n%s", out)
	}
	if !strings.Contains(out, ".mcp.json") {
		t.Errorf("output should mention .mcp.json, got:\n%s", out)
	}

	data, err := os.ReadFile(filepath.Join(repoDir, ".mcp.json"))
	if err != nil {
		t.Fatalf("read .mcp.json: %v", err)
	}
	var cfg map[string]any
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("parse .mcp.json: %v", err)
	}
	servers, ok := cfg["mcpServers"].(map[string]any)
	if !ok {
		t.Fatal("mcpServers missing in .mcp.json")
	}
	if _, ok := servers["rimba"]; !ok {
		t.Error("mcpServers.rimba should be registered in .mcp.json")
	}
}

func TestInitAgentsLocalSkipsMCP(t *testing.T) {
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
	cmd.Flags().Bool(flagAgents, true, "")
	cmd.Flags().Bool(flagLocal, true, "")
	if err := initCmd.RunE(cmd, nil); err != nil {
		t.Fatalf("initCmd.RunE: %v", err)
	}

	out := buf.String()
	if strings.Contains(out, "MCP server:") {
		t.Errorf("--agents --local should not output 'MCP server:' section, got:\n%s", out)
	}
}

func TestInitGlobalMCPErrorPropagated(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	// Write malformed JSON to settings.json — triggers patchJSON error
	claudeDir := filepath.Join(home, ".claude")
	if err := os.MkdirAll(claudeDir, 0750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(claudeDir, "settings.json"), []byte("not json"), 0600); err != nil {
		t.Fatalf("write: %v", err)
	}

	r := &mockRunner{
		run:      func(...string) (string, error) { return "", errors.New("not a git repo") },
		runInDir: noopRunInDir,
	}
	restore := overrideNewRunner(r)
	defer restore()

	cmd, _ := newTestCmd()
	cmd.Flags().Bool(flagGlobal, true, "")
	err := initCmd.RunE(cmd, nil)
	if err == nil {
		t.Fatal("expected error for malformed MCP config, got nil")
	}
	if !strings.Contains(err.Error(), "mcp servers") {
		t.Errorf("error should mention 'mcp servers', got: %v", err)
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

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

	r := repoRootRunner(repoDir, func(args ...string) (string, error) {
		if args[0] == cmdSymbolicRef {
			return refsRemotesOriginMain, nil
		}
		return "", nil
	})
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

	r := repoRootRunner(repoDir, func(_ ...string) (string, error) { return "", nil })
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

	r := repoRootRunner(repoDir, func(_ ...string) (string, error) { return "", nil })
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

	r := repoRootRunner(repoDir, func(_ ...string) (string, error) { return "", nil })
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

	r := repoRootRunner(repoDir, func(args ...string) (string, error) {
		if args[0] == cmdSymbolicRef {
			return refsRemotesOriginMain, nil
		}
		return "", nil
	})
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

	r := repoRootRunner(repoDir, func(args ...string) (string, error) {
		if args[0] == cmdSymbolicRef {
			return refsRemotesOriginMain, nil
		}
		return "", nil
	})
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

	r := repoRootRunner(repoDir, func(args ...string) (string, error) {
		if args[0] == cmdSymbolicRef {
			return refsRemotesOriginMain, nil
		}
		return "", nil
	})
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

	r := repoRootRunner(repoDir, func(_ ...string) (string, error) { return "", nil })
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

	r := repoRootRunner(repoDir, func(args ...string) (string, error) {
		if args[0] == cmdSymbolicRef {
			return refsRemotesOriginMain, nil
		}
		return "", nil
	})
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

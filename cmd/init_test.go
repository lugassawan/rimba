package cmd

import (
	"fmt"
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
			if len(args) >= 1 && args[0] == "symbolic-ref" {
				return "refs/remotes/origin/main", nil
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

	// Verify .rimba.toml was created
	configPath := filepath.Join(repoDir, config.FileName)
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Errorf(".rimba.toml not created at %s", configPath)
	}

	// Verify worktree dir was created
	repoName := filepath.Base(repoDir)
	wtDir := filepath.Join(repoDir, "../"+repoName+"-worktrees")
	if _, err := os.Stat(wtDir); os.IsNotExist(err) {
		t.Errorf("worktree dir not created at %s", wtDir)
	}
}

func TestInitAlreadyExists(t *testing.T) {
	repoDir := t.TempDir()

	// Create an existing .rimba.toml
	configPath := filepath.Join(repoDir, config.FileName)
	if err := os.WriteFile(configPath, []byte(""), 0600); err != nil {
		t.Fatalf("failed to create test config: %v", err)
	}

	r := &mockRunner{
		run: func(args ...string) (string, error) {
			if len(args) >= 2 && args[1] == cmdShowToplevel {
				return repoDir, nil
			}
			if len(args) >= 1 && args[0] == "symbolic-ref" {
				return "refs/remotes/origin/main", nil
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
		t.Fatal("expected error when .rimba.toml already exists")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Errorf("error = %q, want 'already exists'", err.Error())
	}
}

func TestInitRepoRootError(t *testing.T) {
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			if len(args) >= 2 && args[1] == cmdShowToplevel {
				return "", fmt.Errorf("not a git repository")
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

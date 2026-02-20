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

	// Should succeed (not error) when .rimba.toml already exists
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

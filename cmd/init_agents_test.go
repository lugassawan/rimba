package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lugassawan/rimba/internal/config"
)

type agentFilesWant struct {
	output   []string
	noOutput []string
	files    []string
	noFiles  []string
	marker   bool
}

func assertAgentFilesResult(t *testing.T, repoDir, out string, want agentFilesWant) {
	t.Helper()
	for _, s := range want.output {
		if !strings.Contains(out, s) {
			t.Errorf("output missing %q", s)
		}
	}
	for _, s := range want.noOutput {
		if strings.Contains(out, s) {
			t.Errorf("output should not contain %q", s)
		}
	}
	for _, f := range want.files {
		if _, err := os.Stat(filepath.Join(repoDir, f)); os.IsNotExist(err) {
			t.Errorf("file not created: %s", f)
		}
	}
	for _, f := range want.noFiles {
		if _, err := os.Stat(filepath.Join(repoDir, f)); err == nil {
			t.Errorf("file should not exist: %s", f)
		}
	}
	if want.marker {
		content, err := os.ReadFile(filepath.Join(repoDir, "AGENTS.md"))
		if err != nil {
			t.Fatalf("read AGENTS.md: %v", err)
		}
		if strings.Count(string(content), "<!-- BEGIN RIMBA -->") != 1 {
			t.Error("AGENTS.md should have exactly one BEGIN RIMBA marker after re-init")
		}
	}
}

func TestInitAgentFiles(t *testing.T) {
	agentFilePaths := []string{
		"AGENTS.md",
		filepath.Join(".github", "copilot-instructions.md"),
		filepath.Join(".cursor", "rules", "rimba.mdc"),
		filepath.Join(".claude", "skills", "rimba", "SKILL.md"),
	}

	tests := []struct {
		name        string
		withFlag    bool
		preExisting bool
		runTwice    bool
		want        agentFilesWant
	}{
		{
			name:     "creates agent files",
			withFlag: true,
			want:     agentFilesWant{output: agentFilePaths, files: agentFilePaths},
		},
		{
			name:        "installs into existing config",
			withFlag:    true,
			preExisting: true,
			want:        agentFilesWant{output: []string{"already exists"}, files: []string{"AGENTS.md"}},
		},
		{
			name:     "idempotent on second run",
			withFlag: true,
			runTwice: true,
			want:     agentFilesWant{output: []string{"already exists"}, marker: true},
		},
		{
			name: "skips agent files without flag",
			want: agentFilesWant{noOutput: []string{"Agent:"}, noFiles: []string{"AGENTS.md"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repoDir := t.TempDir()
			restore := overrideNewRunner(repoRootRunner(repoDir, func(args ...string) (string, error) {
				if args[0] == cmdSymbolicRef {
					return refsRemotesOriginMain, nil
				}
				return "", nil
			}))
			defer restore()

			if tt.preExisting {
				if err := os.MkdirAll(filepath.Join(repoDir, config.DirName), 0755); err != nil {
					t.Fatal(err)
				}
			}
			if tt.runTwice {
				cmd1, _ := newTestCmd()
				cmd1.Flags().Bool(flagAgents, true, "")
				if err := initCmd.RunE(cmd1, nil); err != nil {
					t.Fatalf("first initCmd.RunE: %v", err)
				}
			}

			cmd, buf := newTestCmd()
			if tt.withFlag {
				cmd.Flags().Bool(flagAgents, true, "")
			}
			if err := initCmd.RunE(cmd, nil); err != nil {
				t.Fatalf("initCmd.RunE: %v", err)
			}
			assertAgentFilesResult(t, repoDir, buf.String(), tt.want)
		})
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

			r := repoRootRunner(repoDir, func(args ...string) (string, error) {
				if args[0] == cmdSymbolicRef {
					return refsRemotesOriginMain, nil
				}
				return "", nil
			})
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

func TestInitAgentsLocalAddsGitignore(t *testing.T) {
	repoDir := t.TempDir()

	r := repoRootRunner(repoDir, func(args ...string) (string, error) {
		if args[0] == cmdSymbolicRef {
			return refsRemotesOriginMain, nil
		}
		return "", nil
	})
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

	r := repoRootRunner(repoDir, func(args ...string) (string, error) {
		if args[0] == cmdSymbolicRef {
			return refsRemotesOriginMain, nil
		}
		return "", nil
	})
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

	r := repoRootRunner(repoDir, func(args ...string) (string, error) {
		if args[0] == cmdSymbolicRef {
			return refsRemotesOriginMain, nil
		}
		return "", nil
	})
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

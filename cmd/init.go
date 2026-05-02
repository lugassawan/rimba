package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/lugassawan/rimba/internal/agentfile"
	"github.com/lugassawan/rimba/internal/config"
	"github.com/lugassawan/rimba/internal/errhint"
	"github.com/lugassawan/rimba/internal/fileutil"
	"github.com/lugassawan/rimba/internal/git"
	"github.com/spf13/cobra"
)

const (
	flagPersonal  = "personal"
	flagGlobal    = "global"
	flagAgents    = "agents"
	flagLocal     = "local"
	flagUninstall = "uninstall"
)

type installTier string

const (
	tierUser         installTier = "user"
	tierProject      installTier = "project"
	tierProjectLocal installTier = "project-local"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize rimba in the current repository",
	Long: `Detects the repository root and sets up the .rimba/ config directory with
settings.toml (team-shared) and settings.local.toml (personal overrides).

If a legacy .rimba.toml file exists, it is migrated into the new directory layout.
Use --agents to also install AI agent instruction files at project level (committed).
Use --agents --local to install them gitignored (personal overrides).
Use -g to install agent files at user level (~/) — works outside a git repository.
Use -g --uninstall to remove user-level files, --agents --uninstall for project-team files,
or --agents --local --uninstall for project-local files.
Use --personal to gitignore the entire .rimba/ directory instead of just the local
config file. In personal mode, settings.local.toml is not created since the whole
directory is already personal.

When --agents or -g is used, rimba also registers itself as an MCP server (server name:
rimba, command: rimba mcp) in client config files (.mcp.json, .cursor/mcp.json,
~/.claude/settings.json, ~/.codex/config.toml, ~/.gemini/settings.json,
~/.codeium/windsurf/mcp_config.json, ~/.roo/mcp.json). --agents --local does not
register MCP — it only updates agent files. Registration is idempotent.`,
	Annotations: map[string]string{"skipConfig": "true"},
	RunE: func(cmd *cobra.Command, args []string) error {
		m := initModeFromFlags(cmd)
		if err := m.validate(); err != nil {
			return err
		}

		// Global (-g) branch: no repo required.
		if m.global {
			return runInitGlobal(cmd, m.uninstall)
		}

		r := newRunner()

		repoRoot, err := git.RepoRoot(r)
		if err != nil {
			return err
		}

		personal, _ := cmd.Flags().GetBool(flagPersonal)

		dirPath := filepath.Join(repoRoot, config.DirName)
		legacyPath := filepath.Join(repoRoot, config.FileName)
		localEntry := filepath.Join(config.DirName, config.LocalFile)
		dirEntry := config.DirName + "/"

		gitignoreEntry := localEntry
		if personal {
			gitignoreEntry = dirEntry
		}

		switch {
		case dirExists(dirPath):
			// .rimba/ directory already exists
			fmt.Fprintf(cmd.OutOrStdout(), "Config %s already exists, skipping config creation\n", dirPath)

		case fileExists(legacyPath):
			if err := runInitMigrate(cmd, repoRoot, dirPath, legacyPath, gitignoreEntry, personal); err != nil {
				return err
			}

		default:
			if err := runInitFresh(cmd, r, repoRoot, dirPath, gitignoreEntry, personal); err != nil {
				return err
			}
		}

		// Project-level agent files
		if m.agents {
			return runInitAgents(cmd, repoRoot, m.local, m.uninstall)
		}

		return nil
	},
}

func runInitFresh(cmd *cobra.Command, r git.Runner, repoRoot, dirPath, gitignoreEntry string, personal bool) error {
	repoName, err := git.RepoName(r)
	if err != nil {
		return err
	}

	defaultBranch, err := git.DefaultBranch(r)
	if err != nil {
		return err
	}

	cfg := &config.Config{
		CopyFiles: config.DefaultCopyFiles(),
	}

	if err := os.MkdirAll(dirPath, 0750); err != nil {
		return errhint.WithFix(
			fmt.Errorf("failed to create config directory: %w", err),
			"check directory permissions for .rimba/ in the repo root",
		)
	}

	if err := config.Save(filepath.Join(dirPath, config.TeamFile), cfg); err != nil {
		return err
	}

	if !personal {
		if err := os.WriteFile(filepath.Join(dirPath, config.LocalFile), nil, 0600); err != nil {
			return fmt.Errorf("failed to create local config: %w", err)
		}
	}

	wtDir := filepath.Join(repoRoot, config.DefaultWorktreeDir(repoName))
	if err := os.MkdirAll(wtDir, 0750); err != nil {
		return fmt.Errorf("failed to create worktree directory: %w", err)
	}

	added, err := fileutil.EnsureGitignore(repoRoot, gitignoreEntry)
	if err != nil {
		return fmt.Errorf("failed to update .gitignore: %w", err)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Initialized rimba in %s\n", repoRoot)
	fmt.Fprintf(cmd.OutOrStdout(), "  Config:       %s\n", filepath.Join(dirPath, config.TeamFile))
	fmt.Fprintf(cmd.OutOrStdout(), "  Worktree dir: %s\n", wtDir)
	fmt.Fprintf(cmd.OutOrStdout(), "  Source:       %s\n", defaultBranch)
	if added {
		fmt.Fprintf(cmd.OutOrStdout(), "  Gitignore:    %s added to .gitignore\n", gitignoreEntry)
	} else {
		fmt.Fprintf(cmd.OutOrStdout(), "  Gitignore:    %s (already in .gitignore)\n", gitignoreEntry)
	}
	return nil
}

func runInitMigrate(cmd *cobra.Command, repoRoot, dirPath, legacyPath, gitignoreEntry string, personal bool) error {
	if err := os.MkdirAll(dirPath, 0750); err != nil {
		return errhint.WithFix(
			fmt.Errorf("failed to create config directory: %w", err),
			"check directory permissions for .rimba/ in the repo root",
		)
	}

	if err := os.Rename(legacyPath, filepath.Join(dirPath, config.TeamFile)); err != nil {
		return fmt.Errorf("failed to move legacy config: %w", err)
	}

	if !personal {
		if err := os.WriteFile(filepath.Join(dirPath, config.LocalFile), nil, 0600); err != nil {
			return fmt.Errorf("failed to create local config: %w", err)
		}
	}

	_, _ = fileutil.RemoveGitignoreEntry(repoRoot, config.FileName)

	if _, err := fileutil.EnsureGitignore(repoRoot, gitignoreEntry); err != nil {
		return fmt.Errorf("failed to update .gitignore: %w", err)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Migrated rimba config in %s\n", repoRoot)
	fmt.Fprintf(cmd.OutOrStdout(), "  Moved:     %s → %s\n", config.FileName, filepath.Join(config.DirName, config.TeamFile))
	if !personal {
		fmt.Fprintf(cmd.OutOrStdout(), "  Created:   %s\n", filepath.Join(config.DirName, config.LocalFile))
	}
	fmt.Fprintf(cmd.OutOrStdout(), "  Gitignore: updated (%s → %s)\n", config.FileName, gitignoreEntry)
	return nil
}

func runInitAgents(cmd *cobra.Command, repoRoot string, local, uninstall bool) error {
	if uninstall {
		if local {
			return runInstall(cmd, repoRoot, tierProjectLocal, "Removed", agentfile.UninstallLocal, nil)
		}
		return runInstall(cmd, repoRoot, tierProject, "Removed", agentfile.UninstallProject, agentfile.UnregisterMCPProject)
	}
	if local {
		return runInstall(cmd, repoRoot, tierProjectLocal, "Installed", agentfile.InstallLocal, nil)
	}
	return runInstall(cmd, repoRoot, tierProject, "Installed", agentfile.InstallProject, agentfile.RegisterMCPProject)
}

func runInitGlobal(cmd *cobra.Command, uninstall bool) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("resolve home directory: %w", err)
	}
	if uninstall {
		return runInstall(cmd, home, tierUser, "Removed", agentfile.UninstallGlobal, agentfile.UnregisterMCPGlobal)
	}
	return runInstall(cmd, home, tierUser, "Installed", agentfile.InstallGlobal, agentfile.RegisterMCPGlobal)
}

// runInstall runs installFn(dir) and optionally mcpFn(dir), then prints results.
// mcpFn may be nil (local tier skips MCP registration).
func runInstall(
	cmd *cobra.Command,
	dir string,
	tier installTier,
	verb string,
	installFn func(string) ([]agentfile.Result, error),
	mcpFn func(string) ([]agentfile.Result, error),
) error {
	files, err := installFn(dir)
	if err != nil {
		return fmt.Errorf("agent files: %w", err)
	}
	var mcps []agentfile.Result
	if mcpFn != nil {
		mcps, err = mcpFn(dir)
		if err != nil {
			return fmt.Errorf("mcp servers: %w", err)
		}
	}
	fmt.Fprintf(cmd.OutOrStdout(), "%s rimba (%s):\n", verb, tier)
	if len(files) > 0 {
		printSection(cmd, "Agent files", tier, files)
	}
	if len(mcps) > 0 {
		printSection(cmd, "MCP server", tier, mcps)
	}
	return nil
}

func printSection(cmd *cobra.Command, title string, tier installTier, results []agentfile.Result) {
	fmt.Fprintf(cmd.OutOrStdout(), "  %s:\n", title)
	for _, r := range results {
		p := r.RelPath
		if tier == tierUser {
			p = "~/" + p
		}
		fmt.Fprintf(cmd.OutOrStdout(), "    %s (%s)\n", p, r.Action)
	}
}

func init() {
	rootCmd.AddCommand(initCmd)
	initCmd.Flags().Bool(flagPersonal, false, "Gitignore the .rimba/ directory (for solo developers)")
	initCmd.Flags().BoolP(flagGlobal, "g", false, "Install rimba agent files at user level (~/)")
	initCmd.Flags().Bool(flagAgents, false, "Install rimba agent files in this project (committed to repo)")
	initCmd.Flags().Bool(flagLocal, false, "With --agents, install as project-local (gitignored)")
	initCmd.Flags().Bool(flagUninstall, false, "Remove agent files: use with -g, --agents, or --agents --local")
	initCmd.MarkFlagsMutuallyExclusive(flagGlobal, flagAgents)
}

// dirExists returns true if path exists and is a directory.
func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

// fileExists returns true if path exists and is a regular file.
func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

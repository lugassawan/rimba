package cmd

import (
	"errors"
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
directory is already personal.`,
	Annotations: map[string]string{"skipConfig": "true"},
	RunE: func(cmd *cobra.Command, args []string) error {
		global, _ := cmd.Flags().GetBool(flagGlobal)
		agents, _ := cmd.Flags().GetBool(flagAgents)
		local, _ := cmd.Flags().GetBool(flagLocal)
		uninstall, _ := cmd.Flags().GetBool(flagUninstall)

		// Flag validation
		if local && !agents {
			return errhint.WithFix(
				errors.New("--local requires --agents"),
				"run: rimba init --agents --local",
			)
		}
		if uninstall && !global && !agents {
			return errhint.WithFix(
				errors.New("--uninstall requires -g or --agents"),
				"run: rimba init -g --uninstall  OR  rimba init --agents --uninstall",
			)
		}

		// Global (-g) branch: no repo required.
		if global {
			home, err := os.UserHomeDir()
			if err != nil {
				return fmt.Errorf("resolve home directory: %w", err)
			}
			if uninstall {
				return runInstall(cmd, home, "user", "Removed", agentfile.UninstallGlobal, agentfile.UnregisterMCPGlobal)
			}
			return runInstall(cmd, home, "user", "Installed", agentfile.InstallGlobal, agentfile.RegisterMCPGlobal)
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
			// Migrate from legacy .rimba.toml → .rimba/ directory
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

		default:
			// Fresh init
			repoName, err := git.RepoName(r)
			if err != nil {
				return err
			}

			defaultBranch, err := git.DefaultBranch(r)
			if err != nil {
				return err
			}

			// Write minimal config — only copy_files (everything else auto-derived)
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

			// Create the worktree directory using convention
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
		}

		// Project-level agent files
		if agents {
			if uninstall {
				if local {
					return runInstall(cmd, repoRoot, "project-local", "Removed", agentfile.UninstallLocal, nil)
				}
				return runInstall(cmd, repoRoot, "project", "Removed", agentfile.UninstallProject, agentfile.UnregisterMCPProject)
			}
			if local {
				return runInstall(cmd, repoRoot, "project-local", "Installed", agentfile.InstallLocal, nil)
			}
			return runInstall(cmd, repoRoot, "project", "Installed", agentfile.InstallProject, agentfile.RegisterMCPProject)
		}

		return nil
	},
}

// runInstall runs installFn(dir) and optionally mcpFn(dir), then prints results.
// mcpFn may be nil (local tier skips MCP registration).
func runInstall(
	cmd *cobra.Command,
	dir, tier, verb string,
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
	printSection(cmd, "Agent files", tier, files)
	if len(mcps) > 0 {
		printSection(cmd, "MCP server", tier, mcps)
	}
	return nil
}

func printSection(cmd *cobra.Command, title, tier string, results []agentfile.Result) {
	fmt.Fprintf(cmd.OutOrStdout(), "  %s:\n", title)
	for _, r := range results {
		p := r.RelPath
		if tier == "user" {
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

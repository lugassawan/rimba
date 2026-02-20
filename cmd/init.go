package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/lugassawan/rimba/internal/agentfile"
	"github.com/lugassawan/rimba/internal/config"
	"github.com/lugassawan/rimba/internal/fileutil"
	"github.com/lugassawan/rimba/internal/git"
	"github.com/spf13/cobra"
)

const (
	flagAgentFiles = "agent-files"
	flagPersonal   = "personal"
)

func init() {
	rootCmd.AddCommand(initCmd)
	initCmd.Flags().Bool(flagAgentFiles, false, "Install AI agent instruction files (AGENTS.md, copilot, cursor, claude)")
	initCmd.Flags().Bool(flagPersonal, false, "Gitignore the .rimba/ directory (for solo developers)")
}

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize rimba in the current repository",
	Long: `Detects the repository root and sets up the .rimba/ config directory with
settings.toml (team-shared) and settings.local.toml (personal overrides).

If a legacy .rimba.toml file exists, it is migrated into the new directory layout.
Use --agent-files to also install AI agent instruction files.
Use --personal to gitignore the entire .rimba/ directory instead of just the local
config file. In personal mode, settings.local.toml is not created since the whole
directory is already personal.`,
	Annotations: map[string]string{"skipConfig": "true"},
	RunE: func(cmd *cobra.Command, args []string) error {
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
				return fmt.Errorf("failed to create config directory: %w", err)
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

			cfg := config.DefaultConfig(repoName, defaultBranch)

			if err := os.MkdirAll(dirPath, 0750); err != nil {
				return fmt.Errorf("failed to create config directory: %w", err)
			}

			if err := config.Save(filepath.Join(dirPath, config.TeamFile), cfg); err != nil {
				return err
			}

			if !personal {
				if err := os.WriteFile(filepath.Join(dirPath, config.LocalFile), nil, 0600); err != nil {
					return fmt.Errorf("failed to create local config: %w", err)
				}
			}

			// Create the worktree directory
			wtDir := filepath.Join(repoRoot, cfg.WorktreeDir)
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

		// Install agent instruction files if requested
		installAgentFiles, _ := cmd.Flags().GetBool(flagAgentFiles)
		if installAgentFiles {
			results, err := agentfile.Install(repoRoot)
			if err != nil {
				return fmt.Errorf("install agent files: %w", err)
			}
			for _, res := range results {
				fmt.Fprintf(cmd.OutOrStdout(), "  Agent:        %s (%s)\n", res.RelPath, res.Action)
			}
		}

		return nil
	},
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

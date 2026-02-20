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

const flagAgentFiles = "agent-files"

func init() {
	rootCmd.AddCommand(initCmd)
	initCmd.Flags().Bool(flagAgentFiles, false, "Install AI agent instruction files (AGENTS.md, copilot, cursor, claude)")
}

var initCmd = &cobra.Command{
	Use:         "init",
	Short:       "Initialize rimba in the current repository",
	Long:        "Detects the repository root, creates a .rimba.toml config file, and sets up the worktree directory. Use --agent-files to also install AI agent instruction files.",
	Annotations: map[string]string{"skipConfig": "true"},
	RunE: func(cmd *cobra.Command, args []string) error {
		r := newRunner()

		repoRoot, err := git.RepoRoot(r)
		if err != nil {
			return err
		}

		configPath := filepath.Join(repoRoot, config.FileName)
		configExists := false
		if _, err := os.Stat(configPath); err == nil {
			configExists = true
		}

		if !configExists {
			repoName, err := git.RepoName(r)
			if err != nil {
				return err
			}

			defaultBranch, err := git.DefaultBranch(r)
			if err != nil {
				return err
			}

			cfg := config.DefaultConfig(repoName, defaultBranch)

			if err := config.Save(configPath, cfg); err != nil {
				return err
			}

			// Create the worktree directory
			wtDir := filepath.Join(repoRoot, cfg.WorktreeDir)
			if err := os.MkdirAll(wtDir, 0750); err != nil {
				return fmt.Errorf("failed to create worktree directory: %w", err)
			}

			added, err := fileutil.EnsureGitignore(repoRoot, config.FileName)
			if err != nil {
				return fmt.Errorf("failed to update .gitignore: %w", err)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Initialized rimba in %s\n", repoRoot)
			fmt.Fprintf(cmd.OutOrStdout(), "  Config:       %s\n", configPath)
			fmt.Fprintf(cmd.OutOrStdout(), "  Worktree dir: %s\n", wtDir)
			fmt.Fprintf(cmd.OutOrStdout(), "  Source:       %s\n", defaultBranch)
			if added {
				fmt.Fprintf(cmd.OutOrStdout(), "  Gitignore:    %s added to .gitignore\n", config.FileName)
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "  Gitignore:    %s (already in .gitignore)\n", config.FileName)
			}
		} else {
			fmt.Fprintf(cmd.OutOrStdout(), "Config %s already exists, skipping config creation\n", configPath)
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

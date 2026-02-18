package cmd

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/lugassawan/rimba/internal/config"
	"github.com/lugassawan/rimba/internal/fileutil"
	"github.com/lugassawan/rimba/internal/git"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(initCmd)
}

var initCmd = &cobra.Command{
	Use:         "init",
	Short:       "Initialize rimba in the current repository",
	Long:        "Detects the repository root, creates a .rimba.toml config file, and sets up the worktree directory.",
	Annotations: map[string]string{"skipConfig": "true"},
	RunE: func(cmd *cobra.Command, args []string) error {
		r := newRunner()

		repoRoot, err := git.RepoRoot(r)
		if err != nil {
			return err
		}

		repoName, err := git.RepoName(r)
		if err != nil {
			return err
		}

		defaultBranch, err := git.DefaultBranch(r)
		if err != nil {
			return err
		}

		configPath := filepath.Join(repoRoot, config.FileName)
		if _, err := os.Stat(configPath); err == nil {
			return errors.New(".rimba.toml already exists (use a text editor to modify it)")
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
			fmt.Fprintf(cmd.OutOrStdout(), "  Gitignore:   %s added to .gitignore\n", config.FileName)
		} else {
			fmt.Fprintf(cmd.OutOrStdout(), "  Gitignore:   %s (already in .gitignore)\n", config.FileName)
		}

		return nil
	},
}

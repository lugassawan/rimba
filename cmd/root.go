package cmd

import (
	"path/filepath"

	"github.com/lugassawan/rimba/internal/config"
	"github.com/lugassawan/rimba/internal/git"
	"github.com/spf13/cobra"
)

const (
	configFileName      = ".rimba.toml"
	errNoConfig         = "config not loaded (run 'rimba init' first)"
	errWorktreeNotFound = "worktree not found for task %q"
)

var rootCmd = &cobra.Command{
	Use:          "rimba",
	Short:        "Git worktree manager",
	Long:         "Rimba simplifies git worktree management with auto-copying dotfiles, branch naming conventions, and worktree status dashboards.",
	SilenceUsage: true,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// Skip config loading for these commands
		if cmd.Name() == "init" || cmd.Name() == "version" || cmd.Name() == "completion" || cmd.Name() == "clean" || cmd.Name() == "update" {
			return nil
		}

		r := &git.ExecRunner{}
		repoRoot, err := git.RepoRoot(r)
		if err != nil {
			return err
		}

		cfg, err := config.Load(filepath.Join(repoRoot, configFileName))
		if err != nil {
			return err
		}
		cmd.SetContext(config.WithConfig(cmd.Context(), cfg))
		return nil
	},
}

func init() {
	rootCmd.PersistentFlags().Bool("no-color", false, "disable colored output")
}

func Execute() error {
	return rootCmd.Execute()
}

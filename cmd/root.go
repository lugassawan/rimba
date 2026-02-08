package cmd

import (
	"path/filepath"

	"github.com/lugassawan/rimba/internal/config"
	"github.com/lugassawan/rimba/internal/git"
	"github.com/spf13/cobra"
)

const (
	configFileName    = ".rimba.toml"
	errNoConfig       = "config not loaded (run 'rimba init' first)"
	errWorktreeNotFmt = "worktree not found for task %q (expected path: %s)"
)

var rootCmd = &cobra.Command{
	Use:          "rimba",
	Short:        "Git worktree manager",
	Long:         "Rimba simplifies git worktree management with auto-copying dotfiles, branch naming conventions, and worktree status dashboards.",
	SilenceUsage: true,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// Skip config loading for init, version, and completion commands
		if cmd.Name() == "init" || cmd.Name() == "version" || cmd.Name() == "completion" || cmd.Name() == "clean" {
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

func Execute() error {
	return rootCmd.Execute()
}

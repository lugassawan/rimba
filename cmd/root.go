package cmd

import (
	"path/filepath"

	"github.com/lugassawan/rimba/internal/config"
	"github.com/lugassawan/rimba/internal/git"
	"github.com/spf13/cobra"
)

const errWorktreeNotFound = "worktree not found for task %q"

var rootCmd = &cobra.Command{
	Use:          "rimba",
	Short:        "Git worktree lifecycle manager",
	Long:         "Rimba manages the full git worktree lifecycle: create, list, rename, duplicate, merge, sync, and clean worktrees with branch naming conventions, dotfile copying, shared dependency management, post-create hooks, and status dashboards.",
	SilenceUsage: true,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// Skip config for Cobra internals (completion, __complete)
		if cmd.Name() == "completion" || cmd.Name() == "__complete" {
			return nil
		}

		// Skip config if any command in the chain is annotated
		for c := cmd; c != nil; c = c.Parent() {
			if c.Annotations != nil && c.Annotations["skipConfig"] == "true" {
				return nil
			}
		}

		r := newRunner()
		repoRoot, err := git.RepoRoot(r)
		if err != nil {
			return err
		}

		cfg, err := config.Load(filepath.Join(repoRoot, config.FileName))
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

package cmd

import (
	"fmt"

	"github.com/lugassawan/rimba/internal/git"
	"github.com/spf13/cobra"
)

func init() {
	cleanCmd.Flags().Bool("dry-run", false, "Show what would be pruned without pruning")
	rootCmd.AddCommand(cleanCmd)
}

var cleanCmd = &cobra.Command{
	Use:   "clean",
	Short: "Prune stale worktree references",
	Long:  "Runs git worktree prune to clean up stale worktree references. Use --dry-run to preview.",
	RunE: func(cmd *cobra.Command, args []string) error {
		r := &git.ExecRunner{}

		dryRun, _ := cmd.Flags().GetBool("dry-run")

		out, err := git.Prune(r, dryRun)
		if err != nil {
			return err
		}

		if out != "" {
			fmt.Fprintln(cmd.OutOrStdout(), out)
		} else if dryRun {
			fmt.Fprintln(cmd.OutOrStdout(), "Nothing to prune.")
		} else {
			fmt.Fprintln(cmd.OutOrStdout(), "Pruned stale worktree references.")
		}

		return nil
	},
}

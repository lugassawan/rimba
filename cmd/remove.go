package cmd

import (
	"fmt"

	"github.com/lugassawan/rimba/internal/git"
	"github.com/lugassawan/rimba/internal/spinner"
	"github.com/spf13/cobra"
)

func init() {
	removeCmd.Flags().Bool("branch", false, "Also delete the local branch")
	removeCmd.Flags().BoolP("force", "f", false, "Force removal even if worktree is dirty")
	rootCmd.AddCommand(removeCmd)
}

var removeCmd = &cobra.Command{
	Use:   "remove <task>",
	Short: "Remove a worktree",
	Long:  "Removes the worktree for the given task. Use --branch to also delete the local branch.",
	Args:  cobra.ExactArgs(1),
	ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if len(args) != 0 {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		return completeWorktreeTasks(cmd, toComplete), cobra.ShellCompDirectiveNoFileComp
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		task := args[0]
		r := newRunner()

		wt, err := findWorktree(r, task)
		if err != nil {
			return err
		}

		s := spinner.New(spinnerOpts(cmd))
		defer s.Stop()

		force, _ := cmd.Flags().GetBool("force")
		s.Start("Removing worktree...")
		if err := git.RemoveWorktree(r, wt.Path, force); err != nil {
			return err
		}

		s.Stop()
		fmt.Fprintf(cmd.OutOrStdout(), "Removed worktree: %s\n", wt.Path)

		deleteBranch, _ := cmd.Flags().GetBool("branch")
		if deleteBranch {
			s.Start("Deleting branch...")
			if err := git.DeleteBranch(r, wt.Branch, force); err != nil {
				s.Stop()
				if force {
					return fmt.Errorf("worktree removed but failed to delete branch %q: %w\nTo force delete: git branch -D %s", wt.Branch, err, wt.Branch)
				}
				return fmt.Errorf("worktree removed but failed to delete branch %q: %w\nTo delete manually: git branch -d %s (or -D to force)", wt.Branch, err, wt.Branch)
			}
			s.Stop()
			fmt.Fprintf(cmd.OutOrStdout(), "Deleted branch: %s\n", wt.Branch)
		}

		return nil
	},
}

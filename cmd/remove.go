package cmd

import (
	"fmt"

	"github.com/lugassawan/rimba/internal/config"
	"github.com/lugassawan/rimba/internal/git"
	"github.com/lugassawan/rimba/internal/resolver"
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
		cfg := config.FromContext(cmd.Context())
		if cfg == nil {
			return fmt.Errorf(errNoConfig)
		}

		r := &git.ExecRunner{}

		// Try to find the worktree by scanning existing worktrees
		entries, err := git.ListWorktrees(r)
		if err != nil {
			return err
		}

		var worktrees []resolver.WorktreeInfo
		for _, e := range entries {
			worktrees = append(worktrees, resolver.WorktreeInfo{
				Path:   e.Path,
				Branch: e.Branch,
			})
		}

		wt, found := resolver.FindBranchForTask(task, worktrees, resolver.AllPrefixes())
		if !found {
			return fmt.Errorf(errWorktreeNotFound, task)
		}

		force, _ := cmd.Flags().GetBool("force")
		if err := git.RemoveWorktree(r, wt.Path, force); err != nil {
			return err
		}

		fmt.Fprintf(cmd.OutOrStdout(), "Removed worktree: %s\n", wt.Path)

		deleteBranch, _ := cmd.Flags().GetBool("branch")
		if deleteBranch {
			if err := git.DeleteBranch(r, wt.Branch, force); err != nil {
				return fmt.Errorf("worktree removed but failed to delete branch %q: %w", wt.Branch, err)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Deleted branch: %s\n", wt.Branch)
		}

		return nil
	},
}

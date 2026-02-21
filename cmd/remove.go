package cmd

import (
	"fmt"

	"github.com/lugassawan/rimba/internal/hint"
	"github.com/lugassawan/rimba/internal/operations"
	"github.com/lugassawan/rimba/internal/spinner"
	"github.com/spf13/cobra"
)

const (
	flagKeepBranch = "keep-branch"

	hintKeepBranch = "Keep the local branch after removing the worktree"
	hintForceRm    = "Force removal even if the worktree has uncommitted changes"
)

func init() {
	removeCmd.Flags().BoolP(flagKeepBranch, "k", false, "Keep the local branch after removing the worktree")
	removeCmd.Flags().BoolP(flagForce, "f", false, "Force removal even if worktree is dirty")
	rootCmd.AddCommand(removeCmd)
}

var removeCmd = &cobra.Command{
	Use:   "remove <task>",
	Short: "Remove a worktree and delete its branch",
	Long:  "Removes the worktree for the given task and deletes the local branch. Use --keep-branch to preserve the branch.",
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

		hint.New(cmd, hintPainter(cmd)).
			Add(flagKeepBranch, hintKeepBranch).
			Add(flagForce, hintForceRm).
			Show()

		keepBranch, _ := cmd.Flags().GetBool(flagKeepBranch)
		force, _ := cmd.Flags().GetBool(flagForce)

		s := spinner.New(spinnerOpts(cmd))
		defer s.Stop()
		s.Start("Removing worktree...")

		result, err := operations.RemoveWorktree(r, wt, task, keepBranch, force, func(msg string) {
			s.Update(msg)
		})
		if err != nil {
			return err
		}

		s.Stop()
		fmt.Fprintf(cmd.OutOrStdout(), "Removed worktree: %s\n", result.Path)

		if result.BranchDeleted {
			fmt.Fprintf(cmd.OutOrStdout(), "Deleted branch: %s\n", result.Branch)
		} else if result.BranchError != nil {
			fmt.Fprintln(cmd.OutOrStdout(), result.BranchError)
		}

		return nil
	},
}

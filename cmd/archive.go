package cmd

import (
	"fmt"

	"github.com/lugassawan/rimba/internal/git"
	"github.com/lugassawan/rimba/internal/hint"
	"github.com/lugassawan/rimba/internal/spinner"
	"github.com/spf13/cobra"
)

const (
	hintForceArchive = "Force archival even if the worktree has uncommitted changes"
)

func init() {
	archiveCmd.Flags().BoolP(flagForce, "f", false, "Force archival even if worktree is dirty")
	rootCmd.AddCommand(archiveCmd)
}

var archiveCmd = &cobra.Command{
	Use:   "archive <task>",
	Short: "Archive a worktree (remove directory, keep branch)",
	Long:  "Removes the worktree directory but preserves the local branch for later restoration with `rimba restore`.",
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
			Add(flagForce, hintForceArchive).
			Show()

		s := spinner.New(spinnerOpts(cmd))
		defer s.Stop()

		force, _ := cmd.Flags().GetBool(flagForce)
		s.Start("Archiving worktree...")
		if err := git.RemoveWorktree(r, wt.Path, force); err != nil {
			return err
		}
		s.Stop()

		out := cmd.OutOrStdout()
		fmt.Fprintf(out, "Archived worktree: %s\n", wt.Path)
		fmt.Fprintf(out, "  Branch preserved: %s\n", wt.Branch)
		fmt.Fprintf(out, "  To restore: rimba restore %s\n", task)
		return nil
	},
}

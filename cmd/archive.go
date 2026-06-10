package cmd

import (
	"fmt"

	"github.com/lugassawan/rimba/internal/hint"
	"github.com/lugassawan/rimba/internal/operations"
	"github.com/lugassawan/rimba/internal/spinner"
	"github.com/spf13/cobra"
)

const (
	hintForceArchive = "Force archival even if the worktree has uncommitted changes"
)

var archiveCmd = &cobra.Command{
	Use:   "archive <task>",
	Short: "Archive a worktree (remove directory, keep branch)",
	Long:  "Removes the worktree directory but preserves the local branch for later restoration with `rimba restore`. Use --dry-run to preview what would be archived without making changes.",
	Example: `  rimba archive auth           # archive worktree, keep branch
  rimba archive auth --dry-run # preview without archiving`,
	Args: cobra.ExactArgs(1),
	ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if len(args) != 0 {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		return completeWorktreeTasks(cmd, toComplete), cobra.ShellCompDirectiveNoFileComp
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		task := args[0]
		r := newRunner()

		wt, err := findWorktree(cmd.Context(), r, task)
		if err != nil {
			return err
		}

		force, _ := cmd.Flags().GetBool(flagForce)
		dryRun, _ := cmd.Flags().GetBool(flagDryRun)

		hint.New(cmd, hintPainter(cmd)).
			Add(flagForce, hintForceArchive).
			Add(flagDryRun, hintDryRun).
			Show()

		s := spinner.New(spinnerOpts(cmd))
		defer s.Stop()

		s.Start("Archiving worktree...")
		result, err := operations.ArchiveWorktree(cmd.Context(), r, operations.ArchiveParams{
			Path:   wt.Path,
			Branch: wt.Branch,
			Force:  force,
			DryRun: dryRun,
		})
		if err != nil {
			return err
		}
		s.Stop()

		if dryRun {
			out := cmd.OutOrStdout()
			for _, step := range result.Plan.Steps {
				fmt.Fprintf(out, "[dry-run] %s\n", step)
			}
			return nil
		}

		out := cmd.OutOrStdout()
		fmt.Fprintf(out, "Archived worktree: %s\n", result.Path)
		fmt.Fprintf(out, "  Branch preserved: %s\n", result.Branch)
		fmt.Fprintf(out, "  To restore: rimba restore %s\n", task)
		return nil
	},
}

func init() {
	archiveCmd.Flags().BoolP(flagForce, "f", false, "force archival even if worktree is dirty")
	archiveCmd.Flags().Bool(flagDryRun, false, "preview what would be archived without making changes")
	rootCmd.AddCommand(archiveCmd)
}

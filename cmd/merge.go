package cmd

import (
	"fmt"

	"github.com/lugassawan/rimba/internal/config"
	"github.com/lugassawan/rimba/internal/git"
	"github.com/lugassawan/rimba/internal/hint"
	"github.com/lugassawan/rimba/internal/operations"
	"github.com/lugassawan/rimba/internal/spinner"
	"github.com/spf13/cobra"
)

const (
	flagInto   = "into"
	flagNoFF   = "no-ff"
	flagKeep   = "keep"
	flagDelete = "delete"

	hintNoFF = "Force a merge commit (preserves branch history in git log)"
	hintKeep = "Keep source worktree after merge (continue working on it)"
	hintInto = "Merge into another worktree instead of the main branch"
)

func init() {
	mergeCmd.Flags().String(flagInto, "", "Target worktree task to merge into (default: main/repo root)")
	mergeCmd.Flags().Bool(flagNoFF, false, "Force a merge commit (no fast-forward)")
	mergeCmd.Flags().Bool(flagKeep, false, "Keep source worktree after merging into main")
	mergeCmd.Flags().Bool(flagDelete, false, "Delete source worktree after merging into another worktree")
	mergeCmd.MarkFlagsMutuallyExclusive(flagKeep, flagDelete)

	_ = mergeCmd.RegisterFlagCompletionFunc(flagInto, func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return completeWorktreeTasks(cmd, toComplete), cobra.ShellCompDirectiveNoFileComp
	})

	rootCmd.AddCommand(mergeCmd)
}

var mergeCmd = &cobra.Command{
	Use:   "merge <source-task>",
	Short: "Merge a worktree branch into main or another worktree",
	Long:  "Merges the source worktree's branch into main (default) or another worktree. Auto-deletes the source when merging to main unless --keep is set. Keeps the source when merging between worktrees unless --delete is set.",
	Args:  cobra.ExactArgs(1),
	ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if len(args) != 0 {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		return completeWorktreeTasks(cmd, toComplete), cobra.ShellCompDirectiveNoFileComp
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		sourceTask := args[0]
		cfg := config.FromContext(cmd.Context())

		r := newRunner()

		repoRoot, err := git.MainRepoRoot(r)
		if err != nil {
			return err
		}

		intoTask, _ := cmd.Flags().GetString(flagInto)
		noFF, _ := cmd.Flags().GetBool(flagNoFF)
		keep, _ := cmd.Flags().GetBool(flagKeep)
		del, _ := cmd.Flags().GetBool(flagDelete)

		hint.New(cmd, hintPainter(cmd)).
			Add(flagNoFF, hintNoFF).
			Add(flagKeep, hintKeep).
			Add(flagInto, hintInto).
			Show()

		s := spinner.New(spinnerOpts(cmd))
		defer s.Stop()

		s.Start("Checking for uncommitted changes...")
		result, err := operations.MergeWorktree(r, operations.MergeParams{
			SourceTask: sourceTask,
			IntoTask:   intoTask,
			RepoRoot:   repoRoot,
			MainBranch: cfg.DefaultSource,
			NoFF:       noFF,
			Keep:       keep,
			Delete:     del,
		}, func(msg string) { s.Update(msg) })
		if err != nil {
			return err
		}

		s.Stop()

		fmt.Fprintf(cmd.OutOrStdout(), "Merged %s into %s\n", result.SourceBranch, result.TargetLabel)

		// Format cleanup results
		if result.SourceRemoved {
			fmt.Fprintf(cmd.OutOrStdout(), "Removed worktree: %s\n", result.SourcePath)
			fmt.Fprintf(cmd.OutOrStdout(), "Deleted branch: %s\n", result.SourceBranch)
		} else if result.WorktreeRemoved {
			fmt.Fprintf(cmd.OutOrStdout(), "Removed worktree: %s\n", result.SourcePath)
			fmt.Fprintf(cmd.OutOrStdout(), "Worktree removed but failed to delete branch: %v\nTo delete manually: git branch -D %s\n", result.RemoveError, result.SourceBranch)
		} else if result.RemoveError != nil {
			fmt.Fprintf(cmd.OutOrStdout(), "Merged successfully but failed to remove worktree: %v\nTo remove manually: rimba remove %s\n", result.RemoveError, sourceTask)
		}

		return nil
	},
}

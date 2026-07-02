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

var mergeCmd = &cobra.Command{
	Use:   "merge <source-task>",
	Short: "Merge a worktree branch into main or another worktree",
	Long:  "Merges the source worktree's branch into main (default) or another worktree. Auto-deletes the source when merging to main unless --keep is set. Keeps the source when merging between worktrees unless --delete is set. Use --dry-run to preview what would happen without making changes.",
	Example: `  rimba merge auth             # merge auth into main
  rimba merge auth --keep      # merge but keep the worktree
  rimba merge auth --dry-run   # preview without merging`,
	Args: cobra.ExactArgs(1),
	ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if len(args) != 0 {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		return completeWorktreeTasks(cmd, toComplete), cobra.ShellCompDirectiveNoFileComp
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg := config.FromContext(cmd.Context())

		r := newRunner(cmd.Context())

		repoRoot, err := git.MainRepoRoot(cmd.Context(), r)
		if err != nil {
			return err
		}

		sourceService, sourceTask := operations.ResolveTaskInput(args[0], repoRoot)
		intoTaskRaw, _ := cmd.Flags().GetString(flagInto)
		var intoService, intoTask string
		if intoTaskRaw != "" {
			intoService, intoTask = operations.ResolveTaskInput(intoTaskRaw, repoRoot)
		}
		noFF, _ := cmd.Flags().GetBool(flagNoFF)
		keep, _ := cmd.Flags().GetBool(flagKeep)
		del, _ := cmd.Flags().GetBool(flagDelete)
		dryRun, _ := cmd.Flags().GetBool(flagDryRun)

		hint.New(cmd, hintPainter(cmd)).
			Add(flagNoFF, hintNoFF).
			Add(flagKeep, hintKeep).
			Add(flagInto, hintInto).
			Add(flagDryRun, hintDryRun).
			Show()

		s := spinner.New(spinnerOpts(cmd))
		defer s.Stop()

		s.Start("Checking for uncommitted changes...")
		result, err := operations.MergeWorktree(cmd.Context(), r, operations.MergeParams{
			SourceTask:    sourceTask,
			SourceService: sourceService,
			IntoTask:      intoTask,
			IntoService:   intoService,
			RepoRoot:      repoRoot,
			MainBranch:    cfg.DefaultSource,
			NoFF:          noFF,
			Keep:          keep,
			Delete:        del,
			DryRun:        dryRun,
		}, func(msg string) { s.Update(msg) })
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

		fmt.Fprintf(cmd.OutOrStdout(), "Merged %s into %s\n", result.SourceBranch, result.TargetLabel)

		// Format cleanup results
		if result.SourceRemoved {
			fmt.Fprintf(cmd.OutOrStdout(), "Removed worktree: %s\n", result.SourcePath)
			fmt.Fprintf(cmd.OutOrStdout(), "Deleted branch: %s\n", result.SourceBranch)
		} else if result.WorktreeRemoved {
			fmt.Fprintf(cmd.OutOrStdout(), "Removed worktree: %s\n", result.SourcePath)
			fmt.Fprintln(cmd.OutOrStdout(), result.RemoveError)
		} else if result.RemoveError != nil {
			fmt.Fprintf(cmd.OutOrStdout(), "Merged successfully but failed to remove worktree: %v\nTo remove manually: rimba remove %s\n", result.RemoveError, sourceTask)
		}

		if result.RemoteDeleted {
			fmt.Fprintf(cmd.OutOrStdout(), "Deleted remote branch: %s/%s\n", git.DefaultRemote, result.SourceBranch)
		} else if result.RemoteError != nil {
			fmt.Fprintf(cmd.OutOrStdout(), "Failed to delete remote branch %s/%s: %v\nTo delete remote: git push %s --delete %s\n",
				git.DefaultRemote, result.SourceBranch, result.RemoteError, git.DefaultRemote, result.SourceBranch)
		}

		return nil
	},
}

func init() {
	mergeCmd.Flags().String(flagInto, "", "target worktree task to merge into (default: main/repo root)")
	mergeCmd.Flags().Bool(flagNoFF, false, "force a merge commit (no fast-forward)")
	mergeCmd.Flags().Bool(flagKeep, false, "keep source worktree after merging into main")
	mergeCmd.Flags().Bool(flagDelete, false, "delete source worktree after merging into another worktree")
	mergeCmd.Flags().Bool(flagDryRun, false, "preview what would be merged/cleaned up without making changes")
	mergeCmd.MarkFlagsMutuallyExclusive(flagKeep, flagDelete)

	_ = mergeCmd.RegisterFlagCompletionFunc(flagInto, func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return completeWorktreeTasks(cmd, toComplete), cobra.ShellCompDirectiveNoFileComp
	})

	rootCmd.AddCommand(mergeCmd)
}

package cmd

import (
	"fmt"

	"github.com/lugassawan/rimba/internal/config"
	"github.com/lugassawan/rimba/internal/errhint"
	"github.com/lugassawan/rimba/internal/hint"
	"github.com/lugassawan/rimba/internal/operations"
	"github.com/lugassawan/rimba/internal/output"
	"github.com/lugassawan/rimba/internal/spinner"
	"github.com/spf13/cobra"
)

const (
	flagKeepBranch = "keep-branch"

	hintKeepBranch = "Keep the local branch after removing the worktree"
	hintForceRm    = "Force removal even if the worktree has uncommitted changes"
)

var removeCmd = &cobra.Command{
	Use:   "remove <task>",
	Short: "Remove a worktree and delete its branch",
	Long:  "Removes the worktree for the given task and deletes the local branch. Use --keep-branch to preserve the branch. Use --dry-run to preview what would be removed without making changes.",
	Example: `  rimba remove auth             # remove worktree and branch
  rimba remove auth --keep-branch  # preserve the local branch
  rimba remove auth --dry-run   # preview without removing`,
	Args: cobra.ExactArgs(1),
	ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if len(args) != 0 {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		return completeWorktreeTasks(cmd, toComplete), cobra.ShellCompDirectiveNoFileComp
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		task := args[0]
		r := newRunner(cmd.Context())
		cfg := config.FromContext(cmd.Context())

		wt, err := findWorktree(cmd.Context(), r, task)
		if err != nil {
			return err
		}

		force, _ := cmd.Flags().GetBool(flagForce)
		if err := operations.GuardKnownPrefix(cfg.PrefixSet(), wt.Branch, cfg.DefaultSource, force); err != nil {
			return err
		}

		if !isJSON(cmd) {
			hint.New(cmd, hintPainter(cmd)).
				Add(flagKeepBranch, hintKeepBranch).
				Add(flagForce, hintForceRm).
				Add(flagDryRun, hintDryRun).
				Show()
		}

		keepBranch, _ := cmd.Flags().GetBool(flagKeepBranch)
		dryRun, _ := cmd.Flags().GetBool(flagDryRun)

		if dryRun {
			if isJSON(cmd) {
				return output.WriteJSON(cmd.OutOrStdout(), version, "remove", output.RemoveData{
					Task:       task,
					Branch:     wt.Branch,
					Path:       wt.Path,
					Prunable:   wt.Prunable,
					KeepBranch: keepBranch,
					DryRun:     true,
				})
			}
			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "[dry-run] would remove worktree: %s\n", wt.Path)
			if !keepBranch {
				fmt.Fprintf(out, "[dry-run] would delete branch: %s\n", wt.Branch)
			}
			return nil
		}

		s := spinner.New(spinnerOpts(cmd))
		defer s.Stop()
		s.Start("Removing worktree...")

		result, err := operations.RemoveWorktree(cmd.Context(), r, wt, task, keepBranch, force, func(msg string) {
			s.Update(msg)
		})
		if err != nil {
			if !force {
				return errhint.WithFix(err, "commit or stash changes, or use --force to discard")
			}
			return err
		}

		s.Stop()
		if isJSON(cmd) {
			return output.WriteJSON(cmd.OutOrStdout(), version, "remove", output.RemoveData{
				Task:            result.Task,
				Branch:          result.Branch,
				Path:            result.Path,
				Prunable:        result.LeftOnDisk,
				WorktreeRemoved: result.WorktreeRemoved,
				BranchDeleted:   result.BranchDeleted,
				KeepBranch:      keepBranch,
				BranchError:     errStr(result.BranchError),
				DryRun:          false,
			})
		}

		if result.LeftOnDisk {
			fmt.Fprintf(cmd.OutOrStdout(), "Cleared stale worktree registration: %s (directory left on disk — remove manually if unneeded)\n", result.Path)
		} else {
			fmt.Fprintf(cmd.OutOrStdout(), "Removed worktree: %s\n", result.Path)
		}

		if result.BranchDeleted {
			fmt.Fprintf(cmd.OutOrStdout(), "Deleted branch: %s\n", result.Branch)
		} else if result.BranchError != nil {
			fmt.Fprintln(cmd.OutOrStdout(), result.BranchError)
		}

		return nil
	},
}

func init() {
	removeCmd.Flags().BoolP(flagKeepBranch, "k", false, "keep the local branch after removing the worktree")
	removeCmd.Flags().BoolP(flagForce, "f", false, "force removal even if worktree is dirty")
	removeCmd.Flags().Bool(flagDryRun, false, "preview what would be removed without making changes")
	rootCmd.AddCommand(removeCmd)
}

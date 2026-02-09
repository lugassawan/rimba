package cmd

import (
	"fmt"

	"github.com/lugassawan/rimba/internal/config"
	"github.com/lugassawan/rimba/internal/git"
	"github.com/lugassawan/rimba/internal/resolver"
	"github.com/spf13/cobra"
)

func init() {
	mergeCmd.Flags().String("into", "", "Target worktree task to merge into (default: main/repo root)")
	mergeCmd.Flags().Bool("no-ff", false, "Force a merge commit (no fast-forward)")
	mergeCmd.Flags().Bool("keep", false, "Keep source worktree after merging into main")
	mergeCmd.Flags().Bool("delete", false, "Delete source worktree after merging into another worktree")
	mergeCmd.MarkFlagsMutuallyExclusive("keep", "delete")

	_ = mergeCmd.RegisterFlagCompletionFunc("into", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
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
		if cfg == nil {
			return errNoConfig
		}

		r := &git.ExecRunner{}

		repoRoot, err := git.RepoRoot(r)
		if err != nil {
			return err
		}

		worktrees, err := listWorktreeInfos(r)
		if err != nil {
			return err
		}

		prefixes := resolver.AllPrefixes()

		// Resolve source
		source, found := resolver.FindBranchForTask(sourceTask, worktrees, prefixes)
		if !found {
			return fmt.Errorf(errWorktreeNotFound, sourceTask)
		}

		// Resolve target
		intoTask, _ := cmd.Flags().GetString("into")
		var targetDir, targetLabel string
		mergingToMain := intoTask == ""

		if mergingToMain {
			targetDir = repoRoot
			targetLabel = cfg.DefaultSource
		} else {
			target, found := resolver.FindBranchForTask(intoTask, worktrees, prefixes)
			if !found {
				return fmt.Errorf(errWorktreeNotFound, intoTask)
			}
			targetDir = target.Path
			targetLabel = target.Branch
		}

		// Pre-flight: check source is clean
		dirty, err := git.IsDirty(r, source.Path)
		if err != nil {
			return err
		}
		if dirty {
			return fmt.Errorf("source worktree %q has uncommitted changes\nCommit or stash changes before merging: cd %s", sourceTask, source.Path)
		}

		// Pre-flight: check target is clean
		dirty, err = git.IsDirty(r, targetDir)
		if err != nil {
			return err
		}
		if dirty {
			if mergingToMain {
				return fmt.Errorf("target %q has uncommitted changes\nCommit or stash changes before merging: cd %s", targetLabel, targetDir)
			}
			return fmt.Errorf("target worktree %q has uncommitted changes\nCommit or stash changes before merging: cd %s", intoTask, targetDir)
		}

		// Execute merge
		noFF, _ := cmd.Flags().GetBool("no-ff")
		if err := git.Merge(r, targetDir, source.Branch, noFF); err != nil {
			return fmt.Errorf("merge failed: %w\nTo resolve conflicts: cd %s", err, targetDir)
		}

		fmt.Fprintf(cmd.OutOrStdout(), "Merged %s into %s\n", source.Branch, targetLabel)

		// Auto-cleanup
		keep, _ := cmd.Flags().GetBool("keep")
		del, _ := cmd.Flags().GetBool("delete")

		shouldDelete := false
		if mergingToMain {
			shouldDelete = !keep
		} else {
			shouldDelete = del
		}

		if shouldDelete {
			if err := git.RemoveWorktree(r, source.Path, false); err != nil {
				fmt.Fprintf(cmd.OutOrStdout(), "Merged successfully but failed to remove worktree: %v\nTo remove manually: rimba remove %s\n", err, sourceTask)
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Removed worktree: %s\n", source.Path)

			if err := git.DeleteBranch(r, source.Branch, true); err != nil {
				fmt.Fprintf(cmd.OutOrStdout(), "Worktree removed but failed to delete branch: %v\nTo delete manually: git branch -D %s\n", err, source.Branch)
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Deleted branch: %s\n", source.Branch)
		}

		return nil
	},
}

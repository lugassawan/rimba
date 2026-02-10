package cmd

import (
	"fmt"

	"github.com/lugassawan/rimba/internal/config"
	"github.com/lugassawan/rimba/internal/git"
	"github.com/lugassawan/rimba/internal/hint"
	"github.com/lugassawan/rimba/internal/resolver"
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
		intoTask, _ := cmd.Flags().GetString(flagInto)
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

		hint.New(cmd, hintPainter(cmd)).
			Add(flagNoFF, hintNoFF).
			Add(flagKeep, hintKeep).
			Add(flagInto, hintInto).
			Show()

		s := spinner.New(spinnerOpts(cmd))
		defer s.Stop()

		// Execute merge
		noFF, _ := cmd.Flags().GetBool(flagNoFF)
		s.Start("Merging...")
		if err := git.Merge(r, targetDir, source.Branch, noFF); err != nil {
			return fmt.Errorf("merge failed: %w\nTo resolve conflicts: cd %s", err, targetDir)
		}

		s.Stop()
		fmt.Fprintf(cmd.OutOrStdout(), "Merged %s into %s\n", source.Branch, targetLabel)

		// Auto-cleanup
		keep, _ := cmd.Flags().GetBool(flagKeep)
		del, _ := cmd.Flags().GetBool(flagDelete)

		shouldDelete := false
		if mergingToMain {
			shouldDelete = !keep
		} else {
			shouldDelete = del
		}

		if shouldDelete {
			s.Start("Removing worktree...")
			if err := git.RemoveWorktree(r, source.Path, false); err != nil {
				s.Stop()
				fmt.Fprintf(cmd.OutOrStdout(), "Merged successfully but failed to remove worktree: %v\nTo remove manually: rimba remove %s\n", err, sourceTask)
				return nil
			}
			s.Stop()
			fmt.Fprintf(cmd.OutOrStdout(), "Removed worktree: %s\n", source.Path)

			s.Start("Deleting branch...")
			if err := git.DeleteBranch(r, source.Branch, true); err != nil {
				s.Stop()
				fmt.Fprintf(cmd.OutOrStdout(), "Worktree removed but failed to delete branch: %v\nTo delete manually: git branch -D %s\n", err, source.Branch)
				return nil
			}
			s.Stop()
			fmt.Fprintf(cmd.OutOrStdout(), "Deleted branch: %s\n", source.Branch)
		}

		return nil
	},
}

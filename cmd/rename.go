package cmd

import (
	"fmt"
	"path/filepath"

	"github.com/lugassawan/rimba/internal/config"
	"github.com/lugassawan/rimba/internal/git"
	"github.com/lugassawan/rimba/internal/resolver"
	"github.com/spf13/cobra"
)

func init() {
	renameCmd.Flags().BoolP("force", "f", false, "Force rename even if worktree is locked")
	rootCmd.AddCommand(renameCmd)
}

var renameCmd = &cobra.Command{
	Use:   "rename <task> <new-task>",
	Short: "Rename a worktree's task, branch, and directory",
	Long:  "Renames the worktree for the given task, updating its branch name and directory to match the new task name.",
	Args:  cobra.ExactArgs(2),
	ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if len(args) == 0 {
			return completeWorktreeTasks(cmd, toComplete), cobra.ShellCompDirectiveNoFileComp
		}
		return nil, cobra.ShellCompDirectiveNoFileComp
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		task := args[0]
		newTask := args[1]
		cfg := config.FromContext(cmd.Context())
		if cfg == nil {
			return fmt.Errorf(errNoConfig)
		}

		r := &git.ExecRunner{}

		repoRoot, err := git.RepoRoot(r)
		if err != nil {
			return err
		}

		// Find the existing worktree by task name
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

		prefixes := resolver.AllPrefixes()
		wt, found := resolver.FindBranchForTask(task, worktrees, prefixes)
		if !found {
			return fmt.Errorf(errWorktreeNotFound, task)
		}

		_, matchedPrefix := resolver.TaskFromBranch(wt.Branch, prefixes)
		if matchedPrefix == "" {
			matchedPrefix, _ = resolver.PrefixString(resolver.DefaultPrefixType)
		}

		newBranch := resolver.BranchName(matchedPrefix, newTask)

		if git.BranchExists(r, newBranch) {
			return fmt.Errorf("branch %q already exists", newBranch)
		}

		wtDir := filepath.Join(repoRoot, cfg.WorktreeDir)
		newPath := resolver.WorktreePath(wtDir, newBranch)

		force, _ := cmd.Flags().GetBool("force")
		if err := git.MoveWorktree(r, wt.Path, newPath, force); err != nil {
			return err
		}

		if err := git.RenameBranch(r, wt.Branch, newBranch); err != nil {
			return fmt.Errorf("worktree moved but failed to rename branch %q: %w", wt.Branch, err)
		}

		fmt.Fprintf(cmd.OutOrStdout(), "Renamed worktree: %s -> %s\n", task, newTask)
		return nil
	},
}

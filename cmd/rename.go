package cmd

import (
	"fmt"
	"path/filepath"

	"github.com/lugassawan/rimba/internal/config"
	"github.com/lugassawan/rimba/internal/git"
	"github.com/lugassawan/rimba/internal/operations"
	"github.com/lugassawan/rimba/internal/spinner"
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

		r := newRunner()

		repoRoot, err := git.RepoRoot(r)
		if err != nil {
			return err
		}

		wt, err := findWorktree(r, task)
		if err != nil {
			return err
		}

		wtDir := filepath.Join(repoRoot, cfg.WorktreeDir)

		s := spinner.New(spinnerOpts(cmd))
		defer s.Stop()

		force, _ := cmd.Flags().GetBool("force")
		s.Start("Renaming worktree...")

		if _, err := operations.RenameWorktree(r, wt, newTask, wtDir, force); err != nil {
			return err
		}

		s.Stop()
		fmt.Fprintf(cmd.OutOrStdout(), "Renamed worktree: %s -> %s\n", task, newTask)
		return nil
	},
}

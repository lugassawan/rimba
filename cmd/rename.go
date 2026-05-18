package cmd

import (
	"fmt"
	"path/filepath"

	"github.com/lugassawan/rimba/internal/config"
	"github.com/lugassawan/rimba/internal/git"
	"github.com/lugassawan/rimba/internal/hint"
	"github.com/lugassawan/rimba/internal/operations"
	"github.com/lugassawan/rimba/internal/resolver"
	"github.com/lugassawan/rimba/internal/spinner"
	"github.com/spf13/cobra"
)

var renameCmd = &cobra.Command{
	Use:   "rename <task> <new-task>",
	Short: "Rename a worktree's task, branch, and directory",
	Long:  "Renames the worktree for the given task, updating its branch name and directory to match the new task name. Use --skip-deps to skip dependency refresh and --skip-hooks to skip post-rename hooks.",
	Example: `  rimba rename auth auth-v2                    # rename auth worktree
  rimba rename auth auth-v2 --skip-hooks       # skip post-rename hooks
  rimba rename auth auth-v2 --skip-deps        # skip dep refresh`,
	Args: cobra.ExactArgs(2),
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

		repoRoot, err := git.MainRepoRoot(r)
		if err != nil {
			return err
		}

		wt, err := findWorktree(r, task)
		if err != nil {
			return err
		}

		_, task = operations.ResolveTaskInput(task, repoRoot)
		_, newTask = operations.ResolveTaskInput(newTask, repoRoot)

		wtDir := filepath.Join(repoRoot, cfg.WorktreeDir)
		skipDeps, _ := cmd.Flags().GetBool(flagSkipDeps)
		skipHooks, _ := cmd.Flags().GetBool(flagSkipHooks)

		hint.New(cmd, hintPainter(cmd)).
			Add(flagSkipDeps, hintSkipDeps).
			Add(flagSkipHooks, hintSkipHooksRename).
			Show()

		s := spinner.New(spinnerOpts(cmd))
		defer s.Stop()

		force, _ := cmd.Flags().GetBool(flagForce)
		s.Start("Renaming worktree...")

		result, err := operations.RenameWorktree(r, wt, newTask, wtDir, force)
		if err != nil {
			return err
		}

		prefixes := resolver.AllPrefixes()
		svc, _, _ := resolver.ServiceFromBranch(wt.Branch, prefixes)
		var configModules []config.ModuleConfig
		if cfg.Deps != nil {
			configModules = cfg.Deps.Modules
		}
		if _, err := operations.PostRenameSetup(r, operations.PostRenameParams{
			WtPath:        result.NewPath,
			Service:       svc,
			SkipDeps:      skipDeps,
			AutoDetect:    cfg.IsAutoDetectDeps(),
			ConfigModules: configModules,
			SkipHooks:     skipHooks,
			PostRename:    cfg.PostRename,
			Concurrency:   cfg.DepsConcurrency(),
		}, func(msg string) { s.Update(msg) }); err != nil {
			return err
		}

		s.Stop()
		fmt.Fprintf(cmd.OutOrStdout(), "Renamed worktree: %s -> %s\n", task, newTask)
		return nil
	},
}

func init() {
	renameCmd.Flags().BoolP(flagForce, "f", false, "Force rename even if worktree is locked")
	renameCmd.Flags().Bool(flagSkipDeps, false, "Skip dependency refresh after rename")
	renameCmd.Flags().Bool(flagSkipHooks, false, "Skip post-rename hooks")
	rootCmd.AddCommand(renameCmd)
}

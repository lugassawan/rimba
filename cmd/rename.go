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
	Use:   "rename <task> [new-task]",
	Short: "Rename a worktree's task, branch, and directory",
	Long:  "Renames the worktree for the given task, updating its branch name and directory to match the new task name. Use a prefix flag (--bugfix, --hotfix, etc.) to change the branch type. Use --skip-deps to skip dependency refresh and --skip-hooks to skip post-rename hooks.",
	Example: `  rimba rename auth auth-v2                    # rename auth worktree
  rimba rename auth --bugfix                   # retype feature/auth → bugfix/auth
  rimba rename auth auth-v2 --bugfix           # rename and retype in one step
  rimba rename auth auth-v2 --skip-hooks       # skip post-rename hooks
  rimba rename auth auth-v2 --skip-deps        # skip dep refresh`,
	Args: cobra.RangeArgs(1, 2),
	ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if len(args) == 0 {
			return completeWorktreeTasks(cmd, toComplete), cobra.ShellCompDirectiveNoFileComp
		}
		return nil, cobra.ShellCompDirectiveNoFileComp
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		task := args[0]
		newTask := task
		if len(args) == 2 {
			newTask = args[1]
		}
		cfg := config.FromContext(cmd.Context())

		r := newRunner(cmd.Context())

		repoRoot, err := git.MainRepoRoot(cmd.Context(), r)
		if err != nil {
			return err
		}

		wt, err := findWorktree(cmd.Context(), r, task)
		if err != nil {
			return err
		}

		_, newTask = operations.ResolveTaskInput(newTask, repoRoot)

		var newPrefix string
		if hasExplicitPrefixFlag(cmd) {
			newPrefix = resolvedPrefixString(cmd)
		}

		if err := ensureTrust(cmd, repoRoot, cfg); err != nil {
			return err
		}

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

		result, err := operations.RenameWorktree(cmd.Context(), r, operations.RenameParams{
			WT:        wt,
			NewTask:   newTask,
			NewPrefix: newPrefix,
			WtDir:     wtDir,
			Force:     force,
		})
		if err != nil {
			return err
		}

		prefixes := resolver.AllPrefixes()
		svc, _, _ := resolver.ServiceFromBranch(result.NewBranch, prefixes)
		var configModules []config.ModuleConfig
		if cfg.Deps != nil {
			configModules = cfg.Deps.Modules
		}
		if _, err := operations.PostRenameSetup(cmd.Context(), r, operations.PostRenameParams{
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
		fmt.Fprintf(cmd.OutOrStdout(), "Renamed worktree: %s -> %s\n", result.OldBranch, result.NewBranch)
		return nil
	},
}

func init() {
	renameCmd.Flags().BoolP(flagForce, "f", false, "force rename even if worktree is locked")
	renameCmd.Flags().Bool(flagSkipDeps, false, "skip dependency refresh after rename")
	renameCmd.Flags().Bool(flagSkipHooks, false, "skip post-rename hooks")
	addPrefixFlags(renameCmd)
	rootCmd.AddCommand(renameCmd)
}

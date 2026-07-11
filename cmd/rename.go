package cmd

import (
	"fmt"
	"path/filepath"

	"github.com/lugassawan/rimba/internal/config"
	"github.com/lugassawan/rimba/internal/git"
	"github.com/lugassawan/rimba/internal/hint"
	"github.com/lugassawan/rimba/internal/operations"
	"github.com/lugassawan/rimba/internal/output"
	"github.com/lugassawan/rimba/internal/resolver"
	"github.com/lugassawan/rimba/internal/spinner"
	"github.com/spf13/cobra"
)

var renameCmd = &cobra.Command{
	Use:   "rename <task> [new-task]",
	Short: "Rename a worktree's task, branch, and directory",
	Long:  "Renames the worktree for the given task, updating its branch name and directory to match the new task name. Use a prefix flag (--bugfix, --hotfix, etc.) to change the branch type. Use --skip-deps to skip dependency refresh, --skip-hooks to skip post-rename hooks, and --push to publish the renamed branch and delete the old remote branch.",
	Example: `  rimba rename auth auth-v2                    # rename auth worktree
  rimba rename auth --bugfix                   # retype feature/auth → bugfix/auth
  rimba rename auth auth-v2 --bugfix           # rename and retype in one step
  rimba rename auth auth-v2 --skip-hooks       # skip post-rename hooks
  rimba rename auth auth-v2 --skip-deps        # skip dep refresh
  rimba rename auth auth-v2 --push             # rename and publish to remote`,
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

		force, _ := cmd.Flags().GetBool(flagForce)
		if err := operations.GuardKnownPrefix(cfg.PrefixSet(), wt.Branch, cfg.DefaultSource, force); err != nil {
			return err
		}

		_, newTask = operations.ResolveTaskInput(newTask, repoRoot, cfg.PrefixSet())

		var newPrefix string
		if sel := resolvePrefixSelection(cmd); sel.Explicit {
			newPrefix = sel.Prefix
		}

		if err := ensureTrust(cmd, repoRoot, cfg); err != nil {
			return err
		}

		wtDir := filepath.Join(repoRoot, cfg.WorktreeDir)
		skipDeps, _ := cmd.Flags().GetBool(flagSkipDeps)
		skipHooks, _ := cmd.Flags().GetBool(flagSkipHooks)

		if !isJSON(cmd) {
			hint.New(cmd, hintPainter(cmd)).
				Add(flagSkipDeps, hintSkipDeps).
				Add(flagSkipHooks, hintSkipHooksRename).
				Show()
		}

		s := spinner.New(spinnerOpts(cmd))
		defer s.Stop()

		push, _ := cmd.Flags().GetBool(flagPush)
		startMsg := "Renaming worktree..."
		if push {
			startMsg = "Renaming and publishing worktree..."
		}
		s.Start(startMsg)

		result, err := operations.RenameWorktree(cmd.Context(), r, operations.RenameParams{
			WT:        wt,
			NewTask:   newTask,
			NewPrefix: newPrefix,
			WtDir:     wtDir,
			Force:     force,
			Push:      push,
		})
		if err != nil {
			return err
		}

		s.Stop()
		if !isJSON(cmd) {
			fmt.Fprintf(cmd.OutOrStdout(), "Renamed worktree: %s -> %s\n", result.OldBranch, result.NewBranch)
			if push {
				reportRenamePush(cmd, result)
			}
		}

		prefixes := cfg.PrefixSet().Strip()
		svc, _, _ := resolver.ServiceFromBranch(result.NewBranch, prefixes)
		var configModules []config.ModuleConfig
		if cfg.Deps != nil {
			configModules = cfg.Deps.Modules
		}
		s.Start("Running post-rename setup...")
		postResult, err := operations.PostRenameSetup(cmd.Context(), r, operations.PostRenameParams{
			WtPath:        result.NewPath,
			Service:       svc,
			SkipDeps:      skipDeps,
			AutoDetect:    cfg.IsAutoDetectDeps(),
			ConfigModules: configModules,
			SkipHooks:     skipHooks,
			PostRename:    cfg.PostRename,
			Concurrency:   cfg.DepsConcurrency(),
		}, func(msg string) { s.Update(msg) })
		if err != nil {
			return err
		}

		s.Stop()
		if isJSON(cmd) {
			return output.WriteJSON(cmd.OutOrStdout(), version, "rename", output.RenameData{
				OldBranch:      result.OldBranch,
				NewBranch:      result.NewBranch,
				OldPath:        result.OldPath,
				NewPath:        result.NewPath,
				Published:      result.Published,
				PublishError:   errStr(result.PublishError),
				RemoteDeleted:  result.RemoteDeleted,
				RemoteError:    errStr(result.RemoteError),
				RemoteSkipped:  result.RemoteSkipped,
				NoOriginRemote: result.NoOriginRemote,
				Deps:           buildDepResults(postResult.DepsResults),
				Hooks:          buildHookResults(postResult.HookResults),
			})
		}
		return nil
	},
}

// reportRenamePush prints the outcome of --push's publish/delete steps, mirroring
// the remote-cleanup reporting style in cmd/merge.go.
func reportRenamePush(cmd *cobra.Command, result operations.RenameResult) {
	out := cmd.OutOrStdout()

	if result.NoOriginRemote {
		fmt.Fprintf(out, "No %s remote; skipped publishing.\n", git.DefaultRemote)
		return
	}

	switch {
	case result.Published:
		fmt.Fprintf(out, "Published branch: %s/%s\n", git.DefaultRemote, result.NewBranch)
	case result.PublishError != nil:
		fmt.Fprintf(out, "Failed to publish branch %s: %v\nTo publish: git push -u %s %s\n",
			result.NewBranch, result.PublishError, git.DefaultRemote, result.NewBranch)
	}

	switch {
	case result.RemoteDeleted:
		fmt.Fprintf(out, "Deleted remote branch: %s/%s\n", git.DefaultRemote, result.OldBranch)
	case result.RemoteError != nil:
		fmt.Fprintf(out, "Failed to delete remote branch %s/%s: %v\nTo delete remote: git push %s --delete %s\n",
			git.DefaultRemote, result.OldBranch, result.RemoteError, git.DefaultRemote, result.OldBranch)
	}
}

func init() {
	renameCmd.Flags().BoolP(flagForce, "f", false, "force rename even if worktree is locked")
	renameCmd.Flags().Bool(flagSkipDeps, false, "skip dependency refresh after rename")
	renameCmd.Flags().Bool(flagSkipHooks, false, "skip post-rename hooks")
	renameCmd.Flags().Bool(flagPush, false, "publish the renamed branch and delete the old remote branch")
	addPrefixFlags(renameCmd)
	rootCmd.AddCommand(renameCmd)
}

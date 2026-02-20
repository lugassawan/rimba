package cmd

import (
	"fmt"
	"path/filepath"

	"github.com/lugassawan/rimba/internal/config"
	"github.com/lugassawan/rimba/internal/deps"
	"github.com/lugassawan/rimba/internal/fileutil"
	"github.com/lugassawan/rimba/internal/git"
	"github.com/lugassawan/rimba/internal/hint"
	"github.com/lugassawan/rimba/internal/operations"
	"github.com/lugassawan/rimba/internal/resolver"
	"github.com/lugassawan/rimba/internal/spinner"
	"github.com/spf13/cobra"
)

func init() {
	restoreCmd.Flags().Bool(flagSkipDeps, false, "Skip dependency detection and installation")
	restoreCmd.Flags().Bool(flagSkipHooks, false, "Skip post-create hooks")
	rootCmd.AddCommand(restoreCmd)
}

var restoreCmd = &cobra.Command{
	Use:   "restore <task>",
	Short: "Restore an archived worktree from its preserved branch",
	Long:  "Recreates a worktree from a branch that was previously archived with `rimba archive`.",
	Args:  cobra.ExactArgs(1),
	ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if len(args) != 0 {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		return completeArchivedTasks(cmd, toComplete), cobra.ShellCompDirectiveNoFileComp
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		task := args[0]
		cfg := config.FromContext(cmd.Context())

		r := newRunner()

		repoRoot, err := git.MainRepoRoot(r)
		if err != nil {
			return err
		}

		branch, err := operations.FindArchivedBranch(r, task)
		if err != nil {
			return err
		}

		wtDir := filepath.Join(repoRoot, cfg.WorktreeDir)
		wtPath := resolver.WorktreePath(wtDir, branch)

		hint.New(cmd, hintPainter(cmd)).
			Add(flagSkipDeps, hintSkipDeps).
			Add(flagSkipHooks, hintSkipHooks).
			Show()

		s := spinner.New(spinnerOpts(cmd))
		defer s.Stop()

		// Create worktree from existing branch
		s.Start("Restoring worktree...")
		if err := git.AddWorktreeFromBranch(r, wtPath, branch); err != nil {
			return err
		}

		// Copy files
		s.Update("Copying files...")
		copied, err := fileutil.CopyEntries(repoRoot, wtPath, cfg.CopyFiles)
		if err != nil {
			return fmt.Errorf("worktree restored but failed to copy files: %w\nTo retry, manually copy files to: %s", err, wtPath)
		}

		// Dependencies
		wtEntries, _ := git.ListWorktrees(r)
		skipDeps, _ := cmd.Flags().GetBool(flagSkipDeps)
		var depsResults []deps.InstallResult
		if !skipDeps {
			s.Update("Installing dependencies...")
			depsResults = installDeps(r, cfg, wtPath, wtEntries, func(cur, total int, name string) {
				s.Update(fmt.Sprintf("Installing dependencies... (%s) [%d/%d]", name, cur, total))
			})
		}

		// Post-create hooks
		skipHooks, _ := cmd.Flags().GetBool(flagSkipHooks)
		var hookResults []deps.HookResult
		if !skipHooks && len(cfg.PostCreate) > 0 {
			s.Update("Running hooks...")
			hookResults = runHooks(wtPath, cfg.PostCreate, func(cur, total int, name string) {
				s.Update(fmt.Sprintf("Running hooks... (%s) [%d/%d]", name, cur, total))
			})
		}

		s.Stop()

		out := cmd.OutOrStdout()
		fmt.Fprintf(out, "Restored worktree for task %q\n", task)
		fmt.Fprintf(out, "  Branch: %s\n", branch)
		fmt.Fprintf(out, "  Path:   %s\n", wtPath)
		if len(copied) > 0 {
			fmt.Fprintf(out, "  Copied: %v\n", copied)
		}
		if skipped := fileutil.SkippedEntries(cfg.CopyFiles, copied); len(skipped) > 0 {
			fmt.Fprintf(out, "  Skipped (not found): %v\n", skipped)
		}

		printInstallResults(out, depsResults)
		printHookResultsList(out, hookResults)

		return nil
	},
}

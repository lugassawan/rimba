package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/lugassawan/rimba/internal/config"
	"github.com/lugassawan/rimba/internal/deps"
	"github.com/lugassawan/rimba/internal/fileutil"
	"github.com/lugassawan/rimba/internal/git"
	"github.com/lugassawan/rimba/internal/hint"
	"github.com/lugassawan/rimba/internal/resolver"
	"github.com/lugassawan/rimba/internal/spinner"
	"github.com/spf13/cobra"
)

const (
	flagSource = "source"

	hintSource = "Branch from a specific source instead of the default branch"
)

func init() {
	addPrefixFlags(addCmd)
	addCmd.Flags().StringP(flagSource, "s", "", "Source branch to create worktree from (default from config)")
	addCmd.Flags().Bool(flagSkipDeps, false, "Skip dependency detection and installation")
	addCmd.Flags().Bool(flagSkipHooks, false, "Skip post-create hooks")
	_ = addCmd.RegisterFlagCompletionFunc(flagSource, func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return completeBranchNames(cmd, toComplete), cobra.ShellCompDirectiveNoFileComp
	})
	rootCmd.AddCommand(addCmd)
}

var addCmd = &cobra.Command{
	Use:   "add <task>",
	Short: "Create a new worktree for a task",
	Long:  "Creates a new git worktree with a branch named <prefix><task> and copies dotfiles from the repo root.",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		task := args[0]
		cfg := config.FromContext(cmd.Context())

		r := newRunner()

		repoRoot, err := git.RepoRoot(r)
		if err != nil {
			return err
		}

		prefix := resolvedPrefixString(cmd)

		source, _ := cmd.Flags().GetString(flagSource)
		if source == "" {
			source = cfg.DefaultSource
		}

		branch := resolver.BranchName(prefix, task)
		wtDir := filepath.Join(repoRoot, cfg.WorktreeDir)
		wtPath := resolver.WorktreePath(wtDir, branch)

		wtEntries, err := git.ListWorktrees(r)
		if err != nil {
			return err
		}

		// Validate
		if git.BranchExists(r, branch) {
			return fmt.Errorf("branch %q already exists", branch)
		}
		if _, err := os.Stat(wtPath); err == nil {
			return fmt.Errorf("worktree path already exists: %s", wtPath)
		}

		hint.New(cmd, hintPainter(cmd)).
			Add(flagSkipDeps, hintSkipDeps).
			Add(flagSkipHooks, hintSkipHooks).
			Add(flagSource, hintSource).
			Show()

		s := spinner.New(spinnerOpts(cmd))
		defer s.Stop()

		// Create worktree
		s.Start("Creating worktree...")
		if err := git.AddWorktree(r, wtPath, branch, source); err != nil {
			return err
		}

		// Copy files
		s.Update("Copying files...")
		copied, err := fileutil.CopyEntries(repoRoot, wtPath, cfg.CopyFiles)
		if err != nil {
			return fmt.Errorf("worktree created but failed to copy files: %w\nTo retry, manually copy files to: %s\nTo remove the worktree: rimba remove %s", err, wtPath, task)
		}

		// Dependencies
		skipDeps, _ := cmd.Flags().GetBool(flagSkipDeps)
		var depsResults []deps.InstallResult
		if !skipDeps {
			s.Update("Installing dependencies...")
			depsResults = installDeps(r, cfg, wtPath, wtEntries)
		}

		// Post-create hooks
		skipHooks, _ := cmd.Flags().GetBool(flagSkipHooks)
		var hookResults []deps.HookResult
		if !skipHooks && len(cfg.PostCreate) > 0 {
			s.Update("Running hooks...")
			hookResults = runHooks(wtPath, cfg.PostCreate)
		}

		s.Stop()

		out := cmd.OutOrStdout()
		fmt.Fprintf(out, "Created worktree for task %q\n", task)
		fmt.Fprintf(out, "  Branch: %s\n", branch)
		fmt.Fprintf(out, "  Path:   %s\n", wtPath)
		if len(copied) > 0 {
			fmt.Fprintf(out, "  Copied: %v\n", copied)
		}

		printInstallResults(out, depsResults)
		printHookResultsList(out, hookResults)

		return nil
	},
}

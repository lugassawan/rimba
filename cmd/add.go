package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/lugassawan/rimba/internal/config"
	"github.com/lugassawan/rimba/internal/fileutil"
	"github.com/lugassawan/rimba/internal/git"
	"github.com/lugassawan/rimba/internal/resolver"
	"github.com/spf13/cobra"
)

func init() {
	addPrefixFlags(addCmd)
	addCmd.Flags().StringP("source", "s", "", "Source branch to create worktree from (default from config)")
	addCmd.Flags().Bool("skip-deps", false, "Skip dependency detection and installation")
	addCmd.Flags().Bool("skip-hooks", false, "Skip post-create hooks")
	_ = addCmd.RegisterFlagCompletionFunc("source", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
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

		source, _ := cmd.Flags().GetString("source")
		if source == "" {
			source = cfg.DefaultSource
		}

		branch := resolver.BranchName(prefix, task)
		wtDir := filepath.Join(repoRoot, cfg.WorktreeDir)
		wtPath := resolver.WorktreePath(wtDir, branch)

		// Validate
		if git.BranchExists(r, branch) {
			return fmt.Errorf("branch %q already exists", branch)
		}
		if _, err := os.Stat(wtPath); err == nil {
			return fmt.Errorf("worktree path already exists: %s", wtPath)
		}

		// Create worktree
		if err := git.AddWorktree(r, wtPath, branch, source); err != nil {
			return err
		}

		// Copy dotfiles
		copied, err := fileutil.CopyDotfiles(repoRoot, wtPath, cfg.CopyFiles)
		if err != nil {
			return fmt.Errorf("worktree created but failed to copy files: %w\nTo retry, manually copy files to: %s\nTo remove the worktree: rimba remove %s", err, wtPath, task)
		}

		out := cmd.OutOrStdout()
		fmt.Fprintf(out, "Created worktree for task %q\n", task)
		fmt.Fprintf(out, "  Branch: %s\n", branch)
		fmt.Fprintf(out, "  Path:   %s\n", wtPath)
		if len(copied) > 0 {
			fmt.Fprintf(out, "  Copied: %v\n", copied)
		}

		// Dependencies
		skipDeps, _ := cmd.Flags().GetBool("skip-deps")
		if !skipDeps {
			printDepsResults(out, r, cfg, wtPath)
		}

		// Post-create hooks
		skipHooks, _ := cmd.Flags().GetBool("skip-hooks")
		if !skipHooks && len(cfg.PostCreate) > 0 {
			printHookResults(out, wtPath, cfg.PostCreate)
		}

		return nil
	},
}

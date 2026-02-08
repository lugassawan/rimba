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
		if cfg == nil {
			return fmt.Errorf(errNoConfig)
		}

		r := &git.ExecRunner{}

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
			return fmt.Errorf("worktree created but failed to copy files: %w", err)
		}

		fmt.Fprintf(cmd.OutOrStdout(), "Created worktree for task %q\n", task)
		fmt.Fprintf(cmd.OutOrStdout(), "  Branch: %s\n", branch)
		fmt.Fprintf(cmd.OutOrStdout(), "  Path:   %s\n", wtPath)
		if len(copied) > 0 {
			fmt.Fprintf(cmd.OutOrStdout(), "  Copied: %v\n", copied)
		}

		return nil
	},
}

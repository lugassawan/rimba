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
	duplicateCmd.Flags().String("as", "", "Custom name for the duplicate worktree")
	duplicateCmd.Flags().Bool("skip-deps", false, "Skip dependency detection and installation")
	duplicateCmd.Flags().Bool("skip-hooks", false, "Skip post-create hooks")
	rootCmd.AddCommand(duplicateCmd)
}

var duplicateCmd = &cobra.Command{
	Use:   "duplicate <task>",
	Short: "Create a new worktree from an existing worktree",
	Long:  "Creates a new worktree branched from an existing worktree's branch, inheriting its prefix. Auto-suffixes with -1, -2, etc. unless --as is provided.",
	Args:  cobra.ExactArgs(1),
	ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if len(args) != 0 {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		return completeWorktreeTasks(cmd, toComplete), cobra.ShellCompDirectiveNoFileComp
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		task := args[0]
		cfg := config.FromContext(cmd.Context())
		if cfg == nil {
			return errNoConfig
		}

		r := &git.ExecRunner{}

		repoRoot, err := git.RepoRoot(r)
		if err != nil {
			return err
		}

		// Find the source worktree
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

		if wt.Branch == cfg.DefaultSource {
			return fmt.Errorf("cannot duplicate the default branch %q; use 'rimba add' instead", cfg.DefaultSource)
		}

		// Extract prefix from source branch
		_, matchedPrefix := resolver.TaskFromBranch(wt.Branch, prefixes)
		if matchedPrefix == "" {
			matchedPrefix, _ = resolver.PrefixString(resolver.DefaultPrefixType)
		}

		// Determine new task name
		asFlag, _ := cmd.Flags().GetString("as")
		var newTask string
		if asFlag != "" {
			newTask = asFlag
		} else {
			// Auto-suffix: try task-1, task-2, etc.
			for i := 1; ; i++ {
				candidate := fmt.Sprintf("%s-%d", task, i)
				candidateBranch := resolver.BranchName(matchedPrefix, candidate)
				if !git.BranchExists(r, candidateBranch) {
					newTask = candidate
					break
				}
			}
		}

		newBranch := resolver.BranchName(matchedPrefix, newTask)
		wtDir := filepath.Join(repoRoot, cfg.WorktreeDir)
		wtPath := resolver.WorktreePath(wtDir, newBranch)

		// Validate
		if git.BranchExists(r, newBranch) {
			return fmt.Errorf("branch %q already exists", newBranch)
		}
		if _, err := os.Stat(wtPath); err == nil {
			return fmt.Errorf("worktree path already exists: %s", wtPath)
		}

		// Create worktree from source branch
		if err := git.AddWorktree(r, wtPath, newBranch, wt.Branch); err != nil {
			return err
		}

		// Copy dotfiles
		copied, err := fileutil.CopyDotfiles(repoRoot, wtPath, cfg.CopyFiles)
		if err != nil {
			return fmt.Errorf("worktree created but failed to copy files: %w\nTo retry, manually copy files to: %s\nTo remove the worktree: rimba remove %s", err, wtPath, newTask)
		}

		out := cmd.OutOrStdout()
		fmt.Fprintf(out, "Duplicated worktree %q as %q\n", task, newTask)
		fmt.Fprintf(out, "  Branch: %s\n", newBranch)
		fmt.Fprintf(out, "  Path:   %s\n", wtPath)
		if len(copied) > 0 {
			fmt.Fprintf(out, "  Copied: %v\n", copied)
		}

		// Dependencies — prefer cloning from source worktree
		skipDeps, _ := cmd.Flags().GetBool("skip-deps")
		if !skipDeps {
			printDepsResultsPreferSource(out, r, cfg, wtPath, wt.Path)
		}

		// Post-create hooks
		skipHooks, _ := cmd.Flags().GetBool("skip-hooks")
		if !skipHooks && len(cfg.PostCreate) > 0 {
			printHookResults(out, wtPath, cfg.PostCreate)
		}

		return nil
	},
}

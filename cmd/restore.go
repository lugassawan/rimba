package cmd

import (
	"fmt"
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

		repoRoot, err := git.RepoRoot(r)
		if err != nil {
			return err
		}

		branch, err := findArchivedBranch(r, task)
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
		fmt.Fprintf(out, "Restored worktree for task %q\n", task)
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

// findArchivedBranch finds a branch for the given task that is not associated with any active worktree.
func findArchivedBranch(r git.Runner, task string) (string, error) {
	branches, err := git.LocalBranches(r)
	if err != nil {
		return "", fmt.Errorf("list branches: %w", err)
	}

	entries, err := git.ListWorktrees(r)
	if err != nil {
		return "", fmt.Errorf("list worktrees: %w", err)
	}

	// Build set of active worktree branches
	active := make(map[string]bool, len(entries))
	for _, e := range entries {
		if e.Branch != "" {
			active[e.Branch] = true
		}
	}

	prefixes := resolver.AllPrefixes()

	// Try prefix+task combinations first
	for _, p := range prefixes {
		candidate := resolver.BranchName(p, task)
		for _, b := range branches {
			if b == candidate && !active[b] {
				return b, nil
			}
		}
	}

	// Fallback: exact match
	for _, b := range branches {
		if b == task && !active[b] {
			return b, nil
		}
	}

	// Fallback: match by task extraction
	for _, b := range branches {
		if active[b] {
			continue
		}
		t, _ := resolver.TaskFromBranch(b, prefixes)
		if t == task {
			return b, nil
		}
	}

	return "", fmt.Errorf("no archived branch found for task %q\nTo see archived branches: rimba list --archived", task)
}

// findArchivedBranches returns branches that are not associated with any active worktree
// and not the main branch.
func findArchivedBranches(r git.Runner, mainBranch string) ([]string, error) {
	branches, err := git.LocalBranches(r)
	if err != nil {
		return nil, err
	}

	entries, err := git.ListWorktrees(r)
	if err != nil {
		return nil, err
	}

	active := make(map[string]bool, len(entries))
	for _, e := range entries {
		if e.Branch != "" {
			active[e.Branch] = true
		}
	}

	var archived []string
	for _, b := range branches {
		if b == mainBranch || active[b] {
			continue
		}
		archived = append(archived, b)
	}
	return archived, nil
}

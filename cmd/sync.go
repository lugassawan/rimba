package cmd

import (
	"fmt"

	"github.com/lugassawan/rimba/internal/config"
	"github.com/lugassawan/rimba/internal/git"
	"github.com/lugassawan/rimba/internal/resolver"
	"github.com/lugassawan/rimba/internal/spinner"
	"github.com/spf13/cobra"
)

// syncContext bundles shared state for sync operations.
type syncContext struct {
	cmd *cobra.Command
	r   git.Runner
	cfg *config.Config
	s   *spinner.Spinner
}

// syncResult tracks the outcome of syncing multiple worktrees.
type syncResult struct {
	synced, skippedDirty, failed int
	failures                     []string
}

func init() {
	syncCmd.Flags().Bool("all", false, "Sync all eligible worktrees")
	syncCmd.Flags().Bool("merge", false, "Use merge instead of rebase")
	syncCmd.Flags().Bool("include-inherited", false, "Include inherited/duplicate worktrees when using --all")

	rootCmd.AddCommand(syncCmd)
}

var syncCmd = &cobra.Command{
	Use:   "sync [task]",
	Short: "Sync worktree(s) with the main branch",
	Long:  "Rebases (or merges) worktree branches onto the latest main branch. Use --all to sync all eligible worktrees, or specify a single task.",
	Args:  cobra.MaximumNArgs(1),
	ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if len(args) != 0 {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		return completeWorktreeTasks(cmd, toComplete), cobra.ShellCompDirectiveNoFileComp
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg := config.FromContext(cmd.Context())

		r := newRunner()
		all, _ := cmd.Flags().GetBool("all")
		useMerge, _ := cmd.Flags().GetBool("merge")
		includeInherited, _ := cmd.Flags().GetBool("include-inherited")

		if !all && len(args) == 0 {
			return fmt.Errorf("provide a task name or use --all to sync all worktrees")
		}

		s := spinner.New(spinnerOpts(cmd))
		defer s.Stop()

		// Fetch latest from origin (non-fatal if no remote configured)
		s.Start("Fetching from origin...")
		if err := git.Fetch(r, "origin"); err != nil {
			s.Stop()
			fmt.Fprintf(cmd.OutOrStdout(), "Warning: fetch failed (no remote?): continuing with local state\n")
		}

		worktrees, err := listWorktreeInfos(r)
		if err != nil {
			return err
		}

		prefixes := resolver.AllPrefixes()
		sc := syncContext{cmd: cmd, r: r, cfg: cfg, s: s}

		if all {
			return syncAll(sc, worktrees, prefixes, useMerge, includeInherited)
		}
		return syncOne(sc, args[0], worktrees, prefixes, useMerge)
	},
}

func syncOne(sc syncContext, task string, worktrees []resolver.WorktreeInfo, prefixes []string, useMerge bool) error {
	wt, found := resolver.FindBranchForTask(task, worktrees, prefixes)
	if !found {
		return fmt.Errorf(errWorktreeNotFound, task)
	}

	dirty, err := git.IsDirty(sc.r, wt.Path)
	if err != nil {
		return err
	}
	if dirty {
		return fmt.Errorf("worktree %q has uncommitted changes\nCommit or stash changes before syncing: cd %s", task, wt.Path)
	}

	verb := "Rebasing"
	if useMerge {
		verb = "Merging"
	}
	sc.s.Update(fmt.Sprintf("%s onto %s...", verb, sc.cfg.DefaultSource))
	if err := doSync(sc.r, wt.Path, sc.cfg.DefaultSource, useMerge); err != nil {
		return err
	}

	sc.s.Stop()
	fmt.Fprintf(sc.cmd.OutOrStdout(), "%s %s onto %s\n", syncMethodLabel(useMerge), wt.Branch, sc.cfg.DefaultSource)
	return nil
}

func syncAll(sc syncContext, worktrees []resolver.WorktreeInfo, prefixes []string, useMerge, includeInherited bool) error { //nolint:unparam // error return matches RunE contract
	allTasks := collectTasks(worktrees, prefixes)
	eligible := filterEligible(worktrees, prefixes, sc.cfg.DefaultSource, allTasks, includeInherited)

	var res syncResult
	for i, wt := range eligible {
		sc.s.Update(fmt.Sprintf("[%d/%d] Syncing %s...", i+1, len(eligible), wt.Branch))
		syncWorktree(sc.cmd, sc.r, sc.cfg.DefaultSource, wt, useMerge, &res)
	}

	sc.s.Stop()
	printSyncSummary(sc.cmd, sc.cfg.DefaultSource, useMerge, &res)
	return nil
}

func collectTasks(worktrees []resolver.WorktreeInfo, prefixes []string) []string {
	tasks := make([]string, 0, len(worktrees))
	for _, wt := range worktrees {
		task, _ := resolver.TaskFromBranch(wt.Branch, prefixes)
		tasks = append(tasks, task)
	}
	return tasks
}

func filterEligible(worktrees []resolver.WorktreeInfo, prefixes []string, mainBranch string, allTasks []string, includeInherited bool) []resolver.WorktreeInfo {
	var eligible []resolver.WorktreeInfo
	for _, wt := range worktrees {
		if wt.Branch == mainBranch || wt.Branch == "" {
			continue
		}
		task, _ := resolver.TaskFromBranch(wt.Branch, prefixes)
		if !includeInherited && resolver.IsInherited(task, allTasks) {
			continue
		}
		eligible = append(eligible, wt)
	}
	return eligible
}

func syncWorktree(cmd *cobra.Command, r git.Runner, mainBranch string, wt resolver.WorktreeInfo, useMerge bool, res *syncResult) {
	dirty, err := git.IsDirty(r, wt.Path)
	if err != nil {
		fmt.Fprintf(cmd.OutOrStdout(), "Warning: could not check status of %s: %v\n", wt.Branch, err)
		res.skippedDirty++
		return
	}
	if dirty {
		fmt.Fprintf(cmd.OutOrStdout(), "Skipping %s (dirty)\n", wt.Branch)
		res.skippedDirty++
		return
	}

	if err := doSync(r, wt.Path, mainBranch, useMerge); err != nil {
		res.failed++
		verb := "rebase"
		if useMerge {
			verb = "merge"
		}
		res.failures = append(res.failures, fmt.Sprintf("  %s: To resolve: cd %s && git %s %s", wt.Branch, wt.Path, verb, mainBranch))
		return
	}
	res.synced++
}

func printSyncSummary(cmd *cobra.Command, mainBranch string, useMerge bool, res *syncResult) {
	fmt.Fprintf(cmd.OutOrStdout(), "%s %d worktree(s) onto %s", syncMethodLabel(useMerge), res.synced, mainBranch)
	if res.skippedDirty > 0 {
		fmt.Fprintf(cmd.OutOrStdout(), ", %d skipped (dirty)", res.skippedDirty)
	}
	if res.failed > 0 {
		fmt.Fprintf(cmd.OutOrStdout(), ", %d failed (conflict)", res.failed)
	}
	fmt.Fprintln(cmd.OutOrStdout())

	for _, f := range res.failures {
		fmt.Fprintln(cmd.OutOrStdout(), f)
	}
}

func syncMethodLabel(useMerge bool) string {
	if useMerge {
		return "Merged"
	}
	return "Rebased"
}

func doSync(r git.Runner, dir, mainBranch string, useMerge bool) error {
	if useMerge {
		return git.Merge(r, dir, mainBranch, false)
	}
	if err := git.Rebase(r, dir, mainBranch); err != nil {
		// Abort the failed rebase to leave worktree in a clean state
		_ = git.AbortRebase(r, dir)
		return err
	}
	return nil
}

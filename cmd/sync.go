package cmd

import (
	"errors"
	"fmt"
	"sync"

	"github.com/lugassawan/rimba/internal/config"
	"github.com/lugassawan/rimba/internal/git"
	"github.com/lugassawan/rimba/internal/hint"
	"github.com/lugassawan/rimba/internal/operations"
	"github.com/lugassawan/rimba/internal/resolver"
	"github.com/lugassawan/rimba/internal/spinner"
	"github.com/spf13/cobra"
)

const (
	flagAll              = "all"
	flagSyncMerge        = "merge"
	flagIncludeInherited = "include-inherited"
	verbRebase           = "rebase"

	hintAll              = "Sync all eligible worktrees at once"
	hintSyncMerge        = "Use merge instead of rebase (preserves history, creates merge commits)"
	hintIncludeInherited = "Include inherited/duplicate worktrees when using --all"
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
	syncCmd.Flags().Bool(flagAll, false, "Sync all eligible worktrees")
	syncCmd.Flags().Bool(flagSyncMerge, false, "Use merge instead of rebase")
	syncCmd.Flags().Bool(flagIncludeInherited, false, "Include inherited/duplicate worktrees when using --all")

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
		all, _ := cmd.Flags().GetBool(flagAll)
		useMerge, _ := cmd.Flags().GetBool(flagSyncMerge)
		includeInherited, _ := cmd.Flags().GetBool(flagIncludeInherited)

		if !all && len(args) == 0 {
			return errors.New("provide a task name or use --all to sync all worktrees")
		}

		hint.New(cmd, hintPainter(cmd)).
			Add(flagAll, hintAll).
			Add(flagSyncMerge, hintSyncMerge).
			Add(flagIncludeInherited, hintIncludeInherited).
			Show()

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
	if err := operations.SyncBranch(sc.r, wt.Path, sc.cfg.DefaultSource, useMerge); err != nil {
		return err
	}

	sc.s.Stop()
	fmt.Fprintf(sc.cmd.OutOrStdout(), "%s %s onto %s\n", operations.SyncMethodLabel(useMerge), wt.Branch, sc.cfg.DefaultSource)
	return nil
}

func syncAll(sc syncContext, worktrees []resolver.WorktreeInfo, prefixes []string, useMerge, includeInherited bool) error { //nolint:unparam // error return matches RunE contract
	allTasks := operations.CollectTasks(worktrees, prefixes)
	eligible := operations.FilterEligible(worktrees, prefixes, sc.cfg.DefaultSource, allTasks, includeInherited)

	var res syncResult
	var mu sync.Mutex
	var wg sync.WaitGroup
	sem := make(chan struct{}, 4) // bounded: git worktrees share object store

	var completed int
	for _, wt := range eligible {
		wg.Add(1)
		go func(wt resolver.WorktreeInfo) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			syncWorktree(sc.cmd, sc.r, sc.cfg.DefaultSource, wt, useMerge, &res, &mu)

			mu.Lock()
			completed++
			sc.s.Update(fmt.Sprintf("[%d/%d] Syncing worktrees...", completed, len(eligible)))
			mu.Unlock()
		}(wt)
	}
	wg.Wait()

	sc.s.Stop()
	printSyncSummary(sc.cmd, sc.cfg.DefaultSource, useMerge, &res)
	return nil
}

func syncWorktree(cmd *cobra.Command, r git.Runner, mainBranch string, wt resolver.WorktreeInfo, useMerge bool, res *syncResult, mu *sync.Mutex) {
	dirty, err := git.IsDirty(r, wt.Path)
	if err != nil {
		mu.Lock()
		fmt.Fprintf(cmd.OutOrStdout(), "Warning: could not check status of %s: %v\n", wt.Branch, err)
		res.skippedDirty++
		mu.Unlock()
		return
	}
	if dirty {
		mu.Lock()
		fmt.Fprintf(cmd.OutOrStdout(), "Skipping %s (dirty)\n", wt.Branch)
		res.skippedDirty++
		mu.Unlock()
		return
	}

	if err := operations.SyncBranch(r, wt.Path, mainBranch, useMerge); err != nil {
		mu.Lock()
		res.failed++
		verb := verbRebase
		if useMerge {
			verb = flagSyncMerge
		}
		res.failures = append(res.failures, fmt.Sprintf("  %s: To resolve: cd %s && git %s %s", wt.Branch, wt.Path, verb, mainBranch))
		mu.Unlock()
		return
	}
	mu.Lock()
	res.synced++
	mu.Unlock()
}

func printSyncSummary(cmd *cobra.Command, mainBranch string, useMerge bool, res *syncResult) {
	fmt.Fprintf(cmd.OutOrStdout(), "%s %d worktree(s) onto %s", operations.SyncMethodLabel(useMerge), res.synced, mainBranch)
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

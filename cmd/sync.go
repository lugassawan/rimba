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
	flagNoPush           = "no-push"

	hintAll              = "Sync all eligible worktrees at once"
	hintSyncMerge        = "Use merge instead of rebase (preserves history, creates merge commits)"
	hintIncludeInherited = "Include inherited/duplicate worktrees when using --all"
	hintNoPush           = "Skip pushing after sync (useful for local-only rebase/merge)"
)

// syncContext bundles shared state for sync operations.
type syncContext struct {
	cmd *cobra.Command
	r   git.Runner
	cfg *config.Config
	s   *spinner.Spinner
	res *syncResult // used by syncAll goroutines
	mu  sync.Mutex  // guards res and output in syncAll
}

// syncResult tracks the outcome of syncing multiple worktrees.
type syncResult struct {
	synced, skippedDirty, failed    int
	pushed, pushSkipped, pushFailed int
	failures                        []string
}

func init() {
	syncCmd.Flags().Bool(flagAll, false, "Sync all eligible worktrees")
	syncCmd.Flags().Bool(flagSyncMerge, false, "Use merge instead of rebase")
	syncCmd.Flags().Bool(flagIncludeInherited, false, "Include inherited/duplicate worktrees when using --all")
	syncCmd.Flags().Bool(flagNoPush, false, "Skip pushing after sync")

	rootCmd.AddCommand(syncCmd)
}

var syncCmd = &cobra.Command{
	Use:   "sync [task]",
	Short: "Sync worktree(s) with the main branch",
	Long:  "Rebases (or merges) worktree branches onto the latest main branch and pushes the result. Use --no-push to skip pushing. Use --all to sync all eligible worktrees, or specify a single task.",
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
		noPush, _ := cmd.Flags().GetBool(flagNoPush)
		push := !noPush

		if !all && len(args) == 0 {
			return errors.New("provide a task name or use --all to sync all worktrees")
		}

		hint.New(cmd, hintPainter(cmd)).
			Add(flagAll, hintAll).
			Add(flagSyncMerge, hintSyncMerge).
			Add(flagIncludeInherited, hintIncludeInherited).
			Add(flagNoPush, hintNoPush).
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
		sc := &syncContext{cmd: cmd, r: r, cfg: cfg, s: s}

		if all {
			return syncAll(sc, worktrees, prefixes, useMerge, includeInherited, push)
		}
		return syncOne(sc, args[0], worktrees, prefixes, useMerge, push)
	},
}

func syncOne(sc *syncContext, task string, worktrees []resolver.WorktreeInfo, prefixes []string, useMerge, push bool) error {
	wt, found := resolver.FindBranchForTask(task, worktrees, prefixes)
	if !found {
		return fmt.Errorf(operations.ErrWorktreeNotFoundFmt, task)
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

	if push {
		sc.s.Start("Pushing to origin...")
		pushed, _, pushErr := operations.PushBranch(sc.r, wt.Path, useMerge)
		sc.s.Stop()
		if pushErr != nil {
			pushHint := fmt.Sprintf("cd %s && git push --force-with-lease", wt.Path)
			if useMerge {
				pushHint = fmt.Sprintf("cd %s && git push", wt.Path)
			}
			return fmt.Errorf("push failed for %s: %w\nTo resolve: %s", wt.Branch, pushErr, pushHint)
		}
		if pushed {
			fmt.Fprintf(sc.cmd.OutOrStdout(), "Pushed %s to origin\n", wt.Branch)
		}
	}

	return nil
}

func syncAll(sc *syncContext, worktrees []resolver.WorktreeInfo, prefixes []string, useMerge, includeInherited, push bool) error { //nolint:unparam // error return matches RunE contract
	allTasks := operations.CollectTasks(worktrees, prefixes)
	eligible := operations.FilterEligible(worktrees, prefixes, sc.cfg.DefaultSource, allTasks, includeInherited)

	sc.res = &syncResult{}
	var wg sync.WaitGroup
	sem := make(chan struct{}, 4) // bounded: git worktrees share object store

	var completed int
	for _, wt := range eligible {
		wg.Add(1)
		go func(wt resolver.WorktreeInfo) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			syncWorktree(sc, sc.cfg.DefaultSource, wt, useMerge, push)

			sc.mu.Lock()
			completed++
			sc.s.Update(fmt.Sprintf("[%d/%d] Syncing worktrees...", completed, len(eligible)))
			sc.mu.Unlock()
		}(wt)
	}
	wg.Wait()

	sc.s.Stop()
	printSyncSummary(sc.cmd, sc.cfg.DefaultSource, useMerge, sc.res)
	return nil
}

func syncWorktree(sc *syncContext, mainBranch string, wt resolver.WorktreeInfo, useMerge, push bool) {
	sr := operations.SyncWorktree(sc.r, mainBranch, wt, useMerge, push)

	sc.mu.Lock()
	defer sc.mu.Unlock()

	switch {
	case sr.Skipped:
		sc.res.skippedDirty++
		if sr.SkipReason == "dirty" {
			fmt.Fprintf(sc.cmd.OutOrStdout(), "Skipping %s (dirty)\n", sr.Branch)
		} else {
			fmt.Fprintf(sc.cmd.OutOrStdout(), "Warning: %s: %s\n", sr.Branch, sr.SkipReason)
		}
	case sr.Failed:
		sc.res.failed++
		sc.res.failures = append(sc.res.failures, fmt.Sprintf("  %s: To resolve: %s", sr.Branch, sr.FailureHint))
	default:
		sc.res.synced++
		if sr.Pushed {
			sc.res.pushed++
		}
		if sr.PushSkipped {
			sc.res.pushSkipped++
		}
		if sr.PushFailed {
			sc.res.pushFailed++
			pushHint := fmt.Sprintf("cd %s && git push --force-with-lease", wt.Path)
			if useMerge {
				pushHint = fmt.Sprintf("cd %s && git push", wt.Path)
			}
			sc.res.failures = append(sc.res.failures, fmt.Sprintf("  %s: push failed: %s\n    To resolve: %s", sr.Branch, sr.PushError, pushHint))
		}
	}
}

func printSyncSummary(cmd *cobra.Command, mainBranch string, useMerge bool, res *syncResult) {
	fmt.Fprintf(cmd.OutOrStdout(), "%s %d worktree(s) onto %s", operations.SyncMethodLabel(useMerge), res.synced, mainBranch)
	if res.pushed > 0 {
		fmt.Fprintf(cmd.OutOrStdout(), ", %d pushed", res.pushed)
	}
	if res.skippedDirty > 0 {
		fmt.Fprintf(cmd.OutOrStdout(), ", %d skipped (dirty)", res.skippedDirty)
	}
	if res.failed > 0 {
		fmt.Fprintf(cmd.OutOrStdout(), ", %d failed (conflict)", res.failed)
	}
	if res.pushFailed > 0 {
		fmt.Fprintf(cmd.OutOrStdout(), ", %d push failed", res.pushFailed)
	}
	fmt.Fprintln(cmd.OutOrStdout())

	for _, f := range res.failures {
		fmt.Fprintln(cmd.OutOrStdout(), f)
	}
}

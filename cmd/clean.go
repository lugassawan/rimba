package cmd

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/lugassawan/rimba/internal/git"
	"github.com/lugassawan/rimba/internal/hint"
	"github.com/lugassawan/rimba/internal/operations"
	"github.com/lugassawan/rimba/internal/resolver"
	"github.com/lugassawan/rimba/internal/spinner"
	"github.com/spf13/cobra"
)

const (
	flagMerged = "merged"
	flagStale  = "stale"

	hintMerged      = "Remove worktrees whose branches are already merged into main"
	hintDryRunPrune = "Preview what would be pruned without making changes"
	hintDryRunClean = "Preview what would be removed without making changes"
	hintForce       = "Skip confirmation and force-remove dirty worktrees"
)

var cleanCmd = &cobra.Command{
	Use:   "clean",
	Short: "Prune stale worktree references or remove merged worktrees",
	Long:  "Runs git worktree prune to clean up stale references and prunes stale remote-tracking refs across all remotes. Use --merged to detect and remove worktrees whose branches have been merged into main.",
	Example: `  rimba clean
  rimba clean --dry-run
  rimba clean --merged
  rimba clean --stale --stale-days 7`,
	Annotations: map[string]string{"skipConfig": "true"},
	RunE: func(cmd *cobra.Command, args []string) error {
		r := newRunner(cmd.Context())
		merged, _ := cmd.Flags().GetBool(flagMerged)
		stale, _ := cmd.Flags().GetBool(flagStale)

		switch {
		case merged:
			return cleanMerged(cmd.Context(), cmd, r)
		case stale:
			return cleanStale(cmd.Context(), cmd, r)
		default:
			return cleanPrune(cmd.Context(), cmd, r)
		}
	},
}

func init() {
	cleanCmd.Flags().Bool(flagDryRun, false, "show what would be pruned/removed without making changes")
	cleanCmd.Flags().Bool(flagMerged, false, "remove worktrees whose branches are merged into main")
	cleanCmd.Flags().Bool(flagStale, false, "remove worktrees with no recent commits")
	cleanCmd.Flags().Int(flagStaleDays, defaultStaleDays, "number of days to consider a worktree stale (used with --stale)")
	cleanCmd.Flags().Bool(flagForce, false, "skip confirmation and force-remove dirty worktrees (with --merged or --stale)")

	cleanCmd.MarkFlagsMutuallyExclusive(flagMerged, flagStale)

	rootCmd.AddCommand(cleanCmd)
}

func cleanPrune(ctx context.Context, cmd *cobra.Command, r git.Runner) error {
	dryRun, _ := cmd.Flags().GetBool(flagDryRun)

	hint.New(cmd, hintPainter(cmd)).
		Add(flagMerged, hintMerged).
		Add(flagDryRun, hintDryRunPrune).
		Show()

	s := spinner.New(spinnerOpts(cmd))
	defer s.Stop()

	s.Start("Pruning stale references...")
	out, err := git.Prune(ctx, r, dryRun)
	if err != nil {
		return err
	}
	s.Stop()

	switch {
	case out != "":
		fmt.Fprintln(cmd.OutOrStdout(), out)
	case dryRun:
		fmt.Fprintln(cmd.OutOrStdout(), "Nothing to prune.")
	default:
		fmt.Fprintln(cmd.OutOrStdout(), "Pruned stale worktree references.")
	}

	return cleanRemotePrune(ctx, cmd, r, s, dryRun)
}

// cleanRemotePrune prunes stale remote-tracking refs across all configured remotes.
// Skips gracefully when there are no remotes; warns and continues on per-remote failure.
func cleanRemotePrune(ctx context.Context, cmd *cobra.Command, r git.Runner, s *spinner.Spinner, dryRun bool) error {
	s.Start("Pruning remote-tracking refs...")
	remotes, err := git.ListRemotes(ctx, r)
	if err != nil {
		s.Stop()
		return err
	}
	if len(remotes) == 0 {
		s.Stop()
		fmt.Fprintln(cmd.OutOrStdout(), "No remotes; skipped remote-ref prune.")
		return nil
	}
	pruned, failures := git.PruneRemotes(ctx, r, remotes, dryRun)
	s.Stop()
	failureMsgs := make([]string, len(failures))
	for i, f := range failures {
		failureMsgs[i] = fmt.Sprintf("failed to prune %s: %v", f.Remote, f.Err)
	}
	printWarnings(cmd, failureMsgs)
	switch {
	case len(pruned) == 0 && len(failures) > 0:
		// All remotes failed; warnings already emitted above.
	case len(pruned) == 0:
		fmt.Fprintln(cmd.OutOrStdout(), "No stale remote-tracking refs to prune.")
	case dryRun:
		fmt.Fprintf(cmd.OutOrStdout(), "Would prune remote-tracking refs: %s\n", strings.Join(pruned, ", "))
	default:
		fmt.Fprintf(cmd.OutOrStdout(), "Pruned remote-tracking refs: %s\n", strings.Join(pruned, ", "))
	}
	return nil
}

// hintEntry is a flag name + hint message pair.
type hintEntry struct {
	flag string
	msg  string
}

// cleanResolveAndHint resolves the main branch and shows relevant flag hints.
func cleanResolveAndHint(ctx context.Context, cmd *cobra.Command, r git.Runner, entries []hintEntry) (string, error) {
	mainBranch, err := resolveMainBranch(ctx, r)
	if err != nil {
		return "", err
	}

	h := hint.New(cmd, hintPainter(cmd))
	for _, e := range entries {
		h.Add(e.flag, e.msg)
	}
	h.Show()

	return mainBranch, nil
}

// cleanStrategy describes what differs between the merged and stale clean modes.
type cleanStrategy struct {
	label         string
	spinnerMsg    string
	emptyMsg      string
	summaryFmt    string
	originPresent bool // pre-resolved: origin exists AND remote deletion is wanted
	// preFind runs before find and owns the spinner; nil means runClean starts it.
	preFind func(*cobra.Command, git.Runner, *spinner.Spinner) error
	find    func(git.Runner) ([]operations.CleanCandidate, []string, error)
	// printRows may ignore the passed slice and use a captured typed one
	// (stale needs []StaleCandidate for age rendering).
	printRows func([]operations.CleanCandidate)
}

// runClean runs the shared clean pipeline using the given strategy.
func runClean(ctx context.Context, cmd *cobra.Command, r git.Runner, s cleanStrategy) error {
	dryRun, _ := cmd.Flags().GetBool(flagDryRun)
	force, _ := cmd.Flags().GetBool(flagForce)

	sp := spinner.New(spinnerOpts(cmd))
	defer sp.Stop()

	if s.preFind != nil {
		if err := s.preFind(cmd, r, sp); err != nil {
			return err
		}
		sp.Update(s.spinnerMsg) // no-op if preFind already stopped the spinner (fetch-fail path)
	} else {
		sp.Start(s.spinnerMsg)
	}

	candidates, warnings, err := s.find(r)
	if err != nil {
		return err
	}
	sp.Stop()

	printWarnings(cmd, warnings)
	if len(candidates) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), s.emptyMsg)
		return nil
	}

	s.printRows(candidates)
	if dryRun {
		return nil
	}
	if !force && !confirmRemoval(cmd, len(candidates), s.label) {
		fmt.Fprintln(cmd.OutOrStdout(), "Aborted.")
		return nil
	}

	removed := cleanRemoveCandidates(ctx, cmd, r, sp, candidates, s.originPresent, force)
	fmt.Fprintf(cmd.OutOrStdout(), s.summaryFmt, removed)
	return nil
}

func cleanMerged(ctx context.Context, cmd *cobra.Command, r git.Runner) error {
	mainBranch, err := cleanResolveAndHint(ctx, cmd, r, []hintEntry{
		{flagDryRun, hintDryRunClean},
		{flagForce, hintForce},
	})
	if err != nil {
		return err
	}

	remotePresent := git.RemoteExists(ctx, r, git.DefaultRemote)
	var mergeRef string
	return runClean(ctx, cmd, r, cleanStrategy{
		label:         "merged",
		spinnerMsg:    "Analyzing branches...",
		emptyMsg:      "No merged worktrees found.",
		summaryFmt:    "Cleaned %d merged worktree(s).\n",
		originPresent: remotePresent, // shared with printMergedCandidates — single probe
		preFind: func(c *cobra.Command, rr git.Runner, sp *spinner.Spinner) error {
			var err error
			mergeRef, err = cleanFetchMergeRef(ctx, c, rr, sp, mainBranch)
			return err
		},
		find: func(rr git.Runner) ([]operations.CleanCandidate, []string, error) {
			result, err := operations.FindMergedCandidates(ctx, rr, mergeRef, mainBranch)
			if err != nil {
				return nil, nil, err
			}
			return result.Candidates, result.Warnings, nil
		},
		printRows: func(candidates []operations.CleanCandidate) {
			printMergedCandidates(cmd, candidates, remotePresent)
		},
	})
}

func cleanStale(ctx context.Context, cmd *cobra.Command, r git.Runner) error {
	mainBranch, err := cleanResolveAndHint(ctx, cmd, r, []hintEntry{
		{flagDryRun, hintDryRunClean},
		{flagForce, hintForce},
		{flagStaleDays, "Customize the staleness threshold (default: 14 days)"},
	})
	if err != nil {
		return err
	}

	staleDays, _ := cmd.Flags().GetInt(flagStaleDays)
	var staleCandidates []operations.StaleCandidate
	return runClean(ctx, cmd, r, cleanStrategy{
		label:         "stale",
		spinnerMsg:    "Analyzing worktree activity...",
		emptyMsg:      "No stale worktrees found.",
		summaryFmt:    "Cleaned %d stale worktree(s).\n",
		originPresent: false, // stale mode is local-only
		find: func(rr git.Runner) ([]operations.CleanCandidate, []string, error) {
			result, err := operations.FindStaleCandidates(ctx, rr, mainBranch, staleDays)
			if err != nil {
				return nil, nil, err
			}
			staleCandidates = result.Candidates
			return flattenStaleCandidates(result.Candidates), result.Warnings, nil
		},
		printRows: func(_ []operations.CleanCandidate) {
			printStaleCandidates(cmd, staleCandidates)
		},
	})
}

// cleanFetchMergeRef fetches from origin and returns the ref to diff against.
// Returns an error on cancellation; falls back to mainBranch with a warning on
// other fetch failures (e.g. no remote configured).
func cleanFetchMergeRef(ctx context.Context, cmd *cobra.Command, r git.Runner, s *spinner.Spinner, mainBranch string) (string, error) {
	s.Start("Fetching from " + git.DefaultRemote + "...")
	if err := git.Fetch(ctx, r, git.DefaultRemote, git.FetchArgs{Prune: true}); err != nil {
		s.Stop()
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return "", err
		}
		fmt.Fprintf(cmd.ErrOrStderr(), "Warning: fetch failed (no remote?): continuing with local state\n")
		return mainBranch, nil
	}
	return git.DefaultRemote + "/" + mainBranch, nil
}

func cleanRemoveCandidates(ctx context.Context, cmd *cobra.Command, r git.Runner, s *spinner.Spinner, candidates []operations.CleanCandidate, originPresent bool, force bool) int {
	s.Start("Removing worktrees...")
	items := operations.RemoveCandidates(ctx, r, candidates, originPresent, force, func(msg string) {
		s.Update(msg)
	})
	s.Stop()
	printCleanedItems(cmd, items)
	return countRemoved(items)
}

func flattenStaleCandidates(candidates []operations.StaleCandidate) []operations.CleanCandidate {
	out := make([]operations.CleanCandidate, len(candidates))
	for i, c := range candidates {
		out[i] = c.CleanCandidate
	}
	return out
}

func printMergedCandidates(cmd *cobra.Command, candidates []operations.CleanCandidate, remotePresent bool) {
	prefixes := resolver.AllPrefixes()
	fmt.Fprintln(cmd.OutOrStdout(), "Merged worktrees:")
	for _, c := range candidates {
		task, _ := resolver.PureTaskFromBranch(c.Branch, prefixes)
		fmt.Fprintf(cmd.OutOrStdout(), "  %s (%s)\n", task, c.Branch)
		if remotePresent {
			fmt.Fprintf(cmd.OutOrStdout(), "    will delete remote: %s/%s\n", git.DefaultRemote, c.Branch)
		}
	}
}

func printStaleCandidates(cmd *cobra.Command, candidates []operations.StaleCandidate) {
	prefixes := resolver.AllPrefixes()
	fmt.Fprintln(cmd.OutOrStdout(), "Stale worktrees:")
	for _, c := range candidates {
		task, _ := resolver.PureTaskFromBranch(c.Branch, prefixes)
		age := resolver.FormatAge(c.LastCommit)
		fmt.Fprintf(cmd.OutOrStdout(), "  %s (%s) — last commit: %s\n", task, c.Branch, age)
	}
}

func confirmRemoval(cmd *cobra.Command, count int, label string) bool {
	fmt.Fprintf(cmd.OutOrStdout(), "\nRemove %d %s worktree(s)? [y/N] ", count, label)
	reader := bufio.NewReader(cmd.InOrStdin())
	answer, _ := reader.ReadString('\n')
	answer = strings.TrimSpace(strings.ToLower(answer))
	return answer == "y" || answer == "yes"
}

func printWarnings(cmd *cobra.Command, warnings []string) {
	for _, w := range warnings {
		fmt.Fprintf(cmd.ErrOrStderr(), "Warning: %s\n", w)
	}
}

func printCleanedItems(cmd *cobra.Command, items []operations.CleanedItem) {
	for _, item := range items {
		if !item.WorktreeRemoved {
			fmt.Fprintf(cmd.OutOrStdout(), "Failed to remove worktree %s\nTo remove manually: rimba remove %s\n", item.Branch, item.Branch)
			continue
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Removed worktree: %s\n", item.Path)
		if item.BranchDeleted {
			fmt.Fprintf(cmd.OutOrStdout(), "Deleted branch: %s\n", item.Branch)
		} else if item.Error != nil {
			fmt.Fprintln(cmd.OutOrStdout(), item.Error)
		}
		if item.RemoteDeleted {
			fmt.Fprintf(cmd.OutOrStdout(), "Deleted remote branch: %s/%s\n", git.DefaultRemote, item.Branch)
		} else if item.RemoteError != nil {
			fmt.Fprintf(cmd.OutOrStdout(), "Failed to delete remote branch %s/%s\nTo delete remote: git push %s --delete %s\n",
				git.DefaultRemote, item.Branch, git.DefaultRemote, item.Branch)
		}
	}
}

func countRemoved(items []operations.CleanedItem) int {
	count := 0
	for _, item := range items {
		if item.WorktreeRemoved && item.BranchDeleted {
			count++
		}
	}
	return count
}

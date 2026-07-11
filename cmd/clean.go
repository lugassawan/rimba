package cmd

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/lugassawan/rimba/internal/config"
	"github.com/lugassawan/rimba/internal/git"
	"github.com/lugassawan/rimba/internal/hint"
	"github.com/lugassawan/rimba/internal/operations"
	"github.com/lugassawan/rimba/internal/output"
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
		cmd.SetContext(withBestEffortConfig(cmd))
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

	if !isJSON(cmd) {
		hint.New(cmd, hintPainter(cmd)).
			Add(flagMerged, hintMerged).
			Add(flagDryRun, hintDryRunPrune).
			Show()
	}

	s := spinner.New(spinnerOpts(cmd))
	defer s.Stop()

	s.Start("Pruning stale references...")
	out, err := git.Prune(ctx, r, dryRun)
	if err != nil {
		return err
	}
	s.Stop()

	if !isJSON(cmd) {
		switch {
		case out != "":
			fmt.Fprintln(cmd.OutOrStdout(), out)
		case dryRun:
			fmt.Fprintln(cmd.OutOrStdout(), "Nothing to prune.")
		default:
			fmt.Fprintln(cmd.OutOrStdout(), "Pruned stale worktree references.")
		}
	}

	return cleanRemotePrune(ctx, cmd, r, s, dryRun, out)
}

// cleanRemotePrune prunes stale remote-tracking refs across all configured remotes.
// Skips gracefully when there are no remotes; warns and continues on per-remote failure.
func cleanRemotePrune(ctx context.Context, cmd *cobra.Command, r git.Runner, s *spinner.Spinner, dryRun bool, pruneOut string) error {
	s.Start("Pruning remote-tracking refs...")
	remotes, err := git.ListRemotes(ctx, r)
	if err != nil {
		s.Stop()
		return err
	}
	if len(remotes) == 0 {
		s.Stop()
		if isJSON(cmd) {
			return output.WriteJSON(cmd.OutOrStdout(), version, "clean", output.CleanData{
				Mode:        "prune",
				DryRun:      dryRun,
				PruneOutput: pruneOut,
				NoRemotes:   true,
			})
		}
		fmt.Fprintln(cmd.OutOrStdout(), "No remotes; skipped remote-ref prune.")
		return nil
	}
	pruned, failures := git.PruneRemotes(ctx, r, remotes, dryRun)
	s.Stop()
	failureMsgs := make([]string, len(failures))
	for i, f := range failures {
		failureMsgs[i] = fmt.Sprintf("failed to prune %s: %v", f.Remote, f.Err)
	}

	if isJSON(cmd) {
		return output.WriteJSON(cmd.OutOrStdout(), version, "clean", output.CleanData{
			Mode:              "prune",
			DryRun:            dryRun,
			PruneOutput:       pruneOut,
			RemotePruned:      nonNilStrings(pruned),
			RemotePruneErrors: failureMsgs,
		})
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

	if !isJSON(cmd) {
		h := hint.New(cmd, hintPainter(cmd))
		for _, e := range entries {
			h.Add(e.flag, e.msg)
		}
		h.Show()
	}

	return mainBranch, nil
}

// cleanStrategy describes what differs between the merged and stale clean modes.
type cleanStrategy struct {
	label         string
	spinnerMsg    string
	emptyMsg      string
	summaryFmt    string
	mode          string // "merged" or "stale" — JSON discriminator
	originPresent bool   // pre-resolved: origin exists AND remote deletion is wanted
	// preFind runs before find and owns the spinner; nil means runClean starts it.
	preFind func(*cobra.Command, git.Runner, *spinner.Spinner) error
	find    func(git.Runner) ([]operations.CleanCandidate, []string, error)
	// printRows may ignore the passed slice and use a captured typed one
	// (stale needs []StaleCandidate for age rendering).
	printRows func([]operations.CleanCandidate)
	// jsonRows builds the JSON candidate view in parallel with printRows —
	// stale needs []StaleCandidate for LastCommit, same reason printRows does.
	jsonRows func([]operations.CleanCandidate) []output.CleanCandidateJSON
}

// runClean runs the shared clean pipeline using the given strategy.
func runClean(ctx context.Context, cmd *cobra.Command, r git.Runner, s cleanStrategy) error {
	dryRun, _ := cmd.Flags().GetBool(flagDryRun)
	force, _ := cmd.Flags().GetBool(flagForce)

	// A confident reap does a real os.Remove, so --dry-run must skip it too.
	if !dryRun {
		reapConfidentLocks(ctx, cmd, r)
	}

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

	if isJSON(cmd) {
		return runCleanJSON(ctx, cmd, r, sp, s, cleanFindResult{candidates, warnings, dryRun, force})
	}

	printWarnings(cmd, warnings)
	if len(candidates) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), s.emptyMsg)
		return nil
	}

	s.printRows(candidates)
	if dryRun {
		return nil
	}
	if !force && !confirmRemoval(cmd, len(candidates), s.label+" worktree(s)") {
		fmt.Fprintln(cmd.OutOrStdout(), "Aborted.")
		return nil
	}

	removed := cleanRemoveCandidates(ctx, cmd, r, sp, candidates, s.originPresent, force)
	fmt.Fprintf(cmd.OutOrStdout(), s.summaryFmt, removed)
	return nil
}

// cleanFindResult bundles s.find's output with the flags runCleanJSON needs,
// keeping runCleanJSON's parameter count under the linter's maxparams limit.
type cleanFindResult struct {
	candidates []operations.CleanCandidate
	warnings   []string
	dryRun     bool
	force      bool
}

// runCleanJSON handles the --json branch of runClean: no candidates, dry-run,
// the force gate, and the force-confirmed removal path.
func runCleanJSON(ctx context.Context, cmd *cobra.Command, r git.Runner, sp *spinner.Spinner, s cleanStrategy, res cleanFindResult) error {
	if len(res.candidates) == 0 {
		return output.WriteJSON(cmd.OutOrStdout(), version, "clean", output.CleanData{
			Mode: s.mode, DryRun: res.dryRun,
			Candidates: make([]output.CleanCandidateJSON, 0),
			Cleaned:    make([]output.CleanedItemJSON, 0),
			Warnings:   nonNilStrings(res.warnings),
		})
	}

	rows := s.jsonRows(res.candidates)
	if res.dryRun {
		return output.WriteJSON(cmd.OutOrStdout(), version, "clean", output.CleanData{
			Mode: s.mode, DryRun: true,
			Candidates: rows,
			Cleaned:    make([]output.CleanedItemJSON, 0),
			Warnings:   nonNilStrings(res.warnings),
		})
	}

	if !res.force {
		return errors.New("interactive confirmation required in JSON mode; pass --force")
	}

	items, removed := cleanRemoveCandidatesData(ctx, r, sp, res.candidates, s.originPresent, res.force)
	cleaned := make([]output.CleanedItemJSON, 0, len(items))
	for _, it := range items {
		cleaned = append(cleaned, output.CleanedItemJSON{
			Branch: it.Branch, Path: it.Path, Prunable: it.Prunable,
			WorktreeRemoved: it.WorktreeRemoved, BranchDeleted: it.BranchDeleted,
			RemoteDeleted: it.RemoteDeleted, RemoteError: errStr(it.RemoteError),
			Error: errStr(it.Error),
		})
	}
	return output.WriteJSON(cmd.OutOrStdout(), version, "clean", output.CleanData{
		Mode: s.mode, DryRun: false,
		Candidates:   rows,
		Cleaned:      cleaned,
		CleanedCount: removed,
		Warnings:     nonNilStrings(res.warnings),
	})
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
		mode:          "merged",
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
		jsonRows: func(candidates []operations.CleanCandidate) []output.CleanCandidateJSON {
			prefixes := config.PrefixSetFromContext(cmd.Context()).Strip()
			rows := make([]output.CleanCandidateJSON, 0, len(candidates))
			for _, c := range candidates {
				task, _ := resolver.PureTaskFromBranch(c.Branch, prefixes)
				rows = append(rows, output.CleanCandidateJSON{
					Task: task, Branch: c.Branch, Path: c.Path, Prunable: c.Prunable,
					WillDeleteRemote: remotePresent,
				})
			}
			return rows
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
		mode:          "stale",
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
		jsonRows: func(_ []operations.CleanCandidate) []output.CleanCandidateJSON {
			prefixes := config.PrefixSetFromContext(cmd.Context()).Strip()
			rows := make([]output.CleanCandidateJSON, 0, len(staleCandidates))
			for _, c := range staleCandidates {
				task, _ := resolver.PureTaskFromBranch(c.Branch, prefixes)
				rows = append(rows, output.CleanCandidateJSON{
					Task: task, Branch: c.Branch, Path: c.Path, Prunable: c.Prunable,
					LastCommit: c.LastCommit.UTC().Format(time.RFC3339),
				})
			}
			return rows
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
		if !isJSON(cmd) {
			fmt.Fprintf(cmd.ErrOrStderr(), "Warning: fetch failed (no remote?): continuing with local state\n")
		}
		return mainBranch, nil
	}
	return git.DefaultRemote + "/" + mainBranch, nil
}

func cleanRemoveCandidatesData(ctx context.Context, r git.Runner, s *spinner.Spinner, candidates []operations.CleanCandidate, originPresent bool, force bool) ([]operations.CleanedItem, int) {
	s.Start("Removing worktrees...")
	items := operations.RemoveCandidates(ctx, r, candidates, originPresent, force, func(msg string) {
		s.Update(msg)
	})
	s.Stop()
	return items, countRemoved(items)
}

func cleanRemoveCandidates(ctx context.Context, cmd *cobra.Command, r git.Runner, s *spinner.Spinner, candidates []operations.CleanCandidate, originPresent bool, force bool) int {
	items, removed := cleanRemoveCandidatesData(ctx, r, s, candidates, originPresent, force)
	printCleanedItems(cmd, items)
	return removed
}

func flattenStaleCandidates(candidates []operations.StaleCandidate) []operations.CleanCandidate {
	out := make([]operations.CleanCandidate, len(candidates))
	for i, c := range candidates {
		out[i] = c.CleanCandidate
	}
	return out
}

func printMergedCandidates(cmd *cobra.Command, candidates []operations.CleanCandidate, remotePresent bool) {
	prefixes := config.PrefixSetFromContext(cmd.Context()).Strip()
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
	prefixes := config.PrefixSetFromContext(cmd.Context()).Strip()
	fmt.Fprintln(cmd.OutOrStdout(), "Stale worktrees:")
	for _, c := range candidates {
		task, _ := resolver.PureTaskFromBranch(c.Branch, prefixes)
		age := resolver.FormatAge(c.LastCommit)
		fmt.Fprintf(cmd.OutOrStdout(), "  %s (%s) — last commit: %s\n", task, c.Branch, age)
	}
}

// confirmRemoval prompts "Remove <count> <noun>? [y/N]" and reports whether
// the user answered y/yes. Shared by clean's worktree removal and doctor's
// stale-lock removal — callers supply the full noun phrase (e.g. "merged
// worktree(s)", "stale index.lock file(s)").
func confirmRemoval(cmd *cobra.Command, count int, noun string) bool {
	fmt.Fprintf(cmd.OutOrStdout(), "\nRemove %d %s? [y/N] ", count, noun)
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
			hint := "git worktree remove --force -- " + item.Path
			if item.Prunable {
				hint = "git worktree prune"
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Failed to remove worktree %s\nTo remove manually: %s\n", item.Branch, hint)
			continue
		}
		if item.Prunable {
			fmt.Fprintf(cmd.OutOrStdout(), "Cleared stale worktree registration: %s (directory left on disk — remove manually if unneeded)\n", item.Path)
		} else {
			fmt.Fprintf(cmd.OutOrStdout(), "Removed worktree: %s\n", item.Path)
		}
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

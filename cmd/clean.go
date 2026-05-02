package cmd

import (
	"bufio"
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
	flagDryRun = "dry-run"
	flagMerged = "merged"
	flagStale  = "stale"

	hintMerged      = "Remove worktrees whose branches are already merged into main"
	hintDryRunPrune = "Preview what would be pruned without making changes"
	hintDryRunClean = "Preview what would be removed without making changes"
	hintForce       = "Skip confirmation prompt"
)

var cleanCmd = &cobra.Command{
	Use:         "clean",
	Short:       "Prune stale worktree references or remove merged worktrees",
	Long:        "Runs git worktree prune to clean up stale references. Use --merged to detect and remove worktrees whose branches have been merged into main.",
	Annotations: map[string]string{"skipConfig": "true"},
	RunE: func(cmd *cobra.Command, args []string) error {
		r := newRunner()
		merged, _ := cmd.Flags().GetBool(flagMerged)
		stale, _ := cmd.Flags().GetBool(flagStale)

		switch {
		case merged:
			return cleanMerged(cmd, r)
		case stale:
			return cleanStale(cmd, r)
		default:
			return cleanPrune(cmd, r)
		}
	},
}

func init() {
	cleanCmd.Flags().Bool(flagDryRun, false, "Show what would be pruned/removed without making changes")
	cleanCmd.Flags().Bool(flagMerged, false, "Remove worktrees whose branches are merged into main")
	cleanCmd.Flags().Bool(flagStale, false, "Remove worktrees with no recent commits")
	cleanCmd.Flags().Int(flagStaleDays, defaultStaleDays, "Number of days to consider a worktree stale (used with --stale)")
	cleanCmd.Flags().Bool(flagForce, false, "Skip confirmation prompt when used with --merged or --stale")

	cleanCmd.MarkFlagsMutuallyExclusive(flagMerged, flagStale)

	rootCmd.AddCommand(cleanCmd)
}

func cleanPrune(cmd *cobra.Command, r git.Runner) error {
	dryRun, _ := cmd.Flags().GetBool(flagDryRun)

	hint.New(cmd, hintPainter(cmd)).
		Add(flagMerged, hintMerged).
		Add(flagDryRun, hintDryRunPrune).
		Show()

	s := spinner.New(spinnerOpts(cmd))
	defer s.Stop()

	s.Start("Pruning stale references...")
	out, err := git.Prune(r, dryRun)
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

	return nil
}

// hintEntry is a flag name + hint message pair.
type hintEntry struct {
	flag string
	msg  string
}

// cleanResolveAndHint resolves the main branch and shows relevant flag hints.
func cleanResolveAndHint(cmd *cobra.Command, r git.Runner, entries []hintEntry) (string, error) {
	mainBranch, err := resolveMainBranch(r)
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
	label      string
	spinnerMsg string
	emptyMsg   string
	summaryFmt string
	// preFind runs before find and owns the spinner; nil means runClean starts it.
	preFind func(*cobra.Command, git.Runner, *spinner.Spinner) error
	find    func(git.Runner) ([]operations.CleanCandidate, []string, error)
	// printRows may ignore the passed slice and use a captured typed one
	// (stale needs []StaleCandidate for age rendering).
	printRows func([]operations.CleanCandidate)
}

// runClean runs the shared clean pipeline using the given strategy.
func runClean(cmd *cobra.Command, r git.Runner, s cleanStrategy) error {
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

	removed := cleanRemoveCandidates(cmd, r, sp, candidates)
	fmt.Fprintf(cmd.OutOrStdout(), s.summaryFmt, removed)
	return nil
}

func cleanMerged(cmd *cobra.Command, r git.Runner) error {
	mainBranch, err := cleanResolveAndHint(cmd, r, []hintEntry{
		{flagDryRun, hintDryRunClean},
		{flagForce, hintForce},
	})
	if err != nil {
		return err
	}

	var mergeRef string
	return runClean(cmd, r, cleanStrategy{
		label:      "merged",
		spinnerMsg: "Analyzing branches...",
		emptyMsg:   "No merged worktrees found.",
		summaryFmt: "Cleaned %d merged worktree(s).\n",
		preFind: func(c *cobra.Command, rr git.Runner, sp *spinner.Spinner) error {
			mergeRef = cleanFetchMergeRef(c, rr, sp, mainBranch)
			return nil
		},
		find: func(rr git.Runner) ([]operations.CleanCandidate, []string, error) {
			result, err := operations.FindMergedCandidates(rr, mergeRef, mainBranch)
			if err != nil {
				return nil, nil, err
			}
			return result.Candidates, result.Warnings, nil
		},
		printRows: func(candidates []operations.CleanCandidate) {
			printMergedCandidates(cmd, candidates)
		},
	})
}

func cleanStale(cmd *cobra.Command, r git.Runner) error {
	mainBranch, err := cleanResolveAndHint(cmd, r, []hintEntry{
		{flagDryRun, hintDryRunClean},
		{flagForce, hintForce},
		{flagStaleDays, "Customize the staleness threshold (default: 14 days)"},
	})
	if err != nil {
		return err
	}

	staleDays, _ := cmd.Flags().GetInt(flagStaleDays)
	var staleCandidates []operations.StaleCandidate
	return runClean(cmd, r, cleanStrategy{
		label:      "stale",
		spinnerMsg: "Analyzing worktree activity...",
		emptyMsg:   "No stale worktrees found.",
		summaryFmt: "Cleaned %d stale worktree(s).\n",
		find: func(rr git.Runner) ([]operations.CleanCandidate, []string, error) {
			result, err := operations.FindStaleCandidates(rr, mainBranch, staleDays)
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
// Falls back to mainBranch with a warning if fetch fails.
func cleanFetchMergeRef(cmd *cobra.Command, r git.Runner, s *spinner.Spinner, mainBranch string) string {
	s.Start("Fetching from origin...")
	if err := git.Fetch(r, "origin"); err != nil {
		s.Stop()
		fmt.Fprintf(cmd.OutOrStdout(), "Warning: fetch failed (no remote?): continuing with local state\n")
		return mainBranch
	}
	return "origin/" + mainBranch
}

func cleanRemoveCandidates(cmd *cobra.Command, r git.Runner, s *spinner.Spinner, candidates []operations.CleanCandidate) int {
	s.Start("Removing worktrees...")
	items := operations.RemoveCandidates(r, candidates, func(msg string) {
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

func printMergedCandidates(cmd *cobra.Command, candidates []operations.CleanCandidate) {
	prefixes := resolver.AllPrefixes()
	fmt.Fprintln(cmd.OutOrStdout(), "Merged worktrees:")
	for _, c := range candidates {
		task, _ := resolver.PureTaskFromBranch(c.Branch, prefixes)
		fmt.Fprintf(cmd.OutOrStdout(), "  %s (%s)\n", task, c.Branch)
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
		fmt.Fprintf(cmd.OutOrStdout(), "Warning: %s\n", w)
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
		} else {
			fmt.Fprintf(cmd.OutOrStdout(), "Worktree removed but failed to delete branch\nTo delete manually: git branch -D %s\n", item.Branch)
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

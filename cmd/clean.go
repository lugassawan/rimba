package cmd

import (
	"bufio"
	"fmt"
	"strings"
	"time"

	"github.com/lugassawan/rimba/internal/git"
	"github.com/lugassawan/rimba/internal/hint"
	"github.com/lugassawan/rimba/internal/resolver"
	"github.com/lugassawan/rimba/internal/spinner"
	"github.com/spf13/cobra"
)

const (
	flagDryRun = "dry-run"
	flagMerged = "merged"
	flagStale  = "stale"

	hintMerged       = "Remove worktrees whose branches are already merged into main"
	hintStale        = "Remove worktrees with no recent commits (configure with --stale-days)"
	hintDryRunPrune  = "Preview what would be pruned without making changes"
	hintDryRunMerged = "Preview what would be removed without making changes"
	hintDryRunStale  = "Preview what would be removed without making changes"
	hintForce        = "Skip confirmation prompt"
)

type cleanCandidate struct {
	path   string
	branch string
}

type staleCandidate struct {
	cleanCandidate
	lastCommit time.Time
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

func cleanMerged(cmd *cobra.Command, r git.Runner) error {
	dryRun, _ := cmd.Flags().GetBool(flagDryRun)
	force, _ := cmd.Flags().GetBool(flagForce)

	mainBranch, err := resolveMainBranch(r)
	if err != nil {
		return err
	}

	hint.New(cmd, hintPainter(cmd)).
		Add(flagDryRun, hintDryRunMerged).
		Add(flagForce, hintForce).
		Show()

	s := spinner.New(spinnerOpts(cmd))
	defer s.Stop()

	// Fetch latest (non-fatal)
	mergeRef := mainBranch
	s.Start("Fetching from origin...")
	if err := git.Fetch(r, "origin"); err != nil {
		s.Stop()
		fmt.Fprintf(cmd.OutOrStdout(), "Warning: fetch failed (no remote?): continuing with local state\n")
	} else {
		mergeRef = "origin/" + mainBranch
	}

	s.Update("Analyzing branches...")
	candidates, err := findMergedCandidates(r, mergeRef, mainBranch)
	if err != nil {
		return err
	}

	s.Stop()

	if len(candidates) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "No merged worktrees found.")
		return nil
	}

	printMergedCandidates(cmd, candidates)

	if dryRun {
		return nil
	}

	if !force && !confirmRemoval(cmd, len(candidates), "merged") {
		fmt.Fprintln(cmd.OutOrStdout(), "Aborted.")
		return nil
	}

	removed := removeWorktrees(cmd, r, candidates)
	fmt.Fprintf(cmd.OutOrStdout(), "Cleaned %d merged worktree(s).\n", removed)
	return nil
}

func findMergedCandidates(r git.Runner, mergeRef, mainBranch string) ([]cleanCandidate, error) {
	mergedList, err := git.MergedBranches(r, mergeRef)
	if err != nil {
		return nil, fmt.Errorf("failed to list merged branches: %w", err)
	}

	mergedSet := make(map[string]bool, len(mergedList))
	for _, b := range mergedList {
		mergedSet[b] = true
	}

	entries, err := git.ListWorktrees(r)
	if err != nil {
		return nil, err
	}

	var candidates []cleanCandidate
	for _, e := range git.FilterEntries(entries, mainBranch) {
		if mergedSet[e.Branch] {
			candidates = append(candidates, cleanCandidate{path: e.Path, branch: e.Branch})
			continue
		}

		// Fallback: squash-merge detection
		squashed, err := git.IsSquashMerged(r, mergeRef, e.Branch)
		if err != nil {
			continue
		}
		if squashed {
			candidates = append(candidates, cleanCandidate{path: e.Path, branch: e.Branch})
		}
	}
	return candidates, nil
}

func printMergedCandidates(cmd *cobra.Command, candidates []cleanCandidate) {
	prefixes := resolver.AllPrefixes()
	fmt.Fprintln(cmd.OutOrStdout(), "Merged worktrees:")
	for _, c := range candidates {
		task, _ := resolver.TaskFromBranch(c.branch, prefixes)
		fmt.Fprintf(cmd.OutOrStdout(), "  %s (%s)\n", task, c.branch)
	}
}

func confirmRemoval(cmd *cobra.Command, count int, label string) bool {
	fmt.Fprintf(cmd.OutOrStdout(), "\nRemove %d %s worktree(s)? [y/N] ", count, label)
	reader := bufio.NewReader(cmd.InOrStdin())
	answer, _ := reader.ReadString('\n')
	answer = strings.TrimSpace(strings.ToLower(answer))
	return answer == "y" || answer == "yes"
}

func removeWorktrees(cmd *cobra.Command, r git.Runner, candidates []cleanCandidate) int {
	var removed int
	for _, c := range candidates {
		if err := git.RemoveWorktree(r, c.path, false); err != nil {
			fmt.Fprintf(cmd.OutOrStdout(), "Failed to remove worktree %s: %v\nTo remove manually: rimba remove %s\n", c.branch, err, c.branch)
			continue
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Removed worktree: %s\n", c.path)

		if err := git.DeleteBranch(r, c.branch, true); err != nil {
			fmt.Fprintf(cmd.OutOrStdout(), "Worktree removed but failed to delete branch: %v\nTo delete manually: git branch -D %s\n", err, c.branch)
			continue
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Deleted branch: %s\n", c.branch)
		removed++
	}
	return removed
}

func cleanStale(cmd *cobra.Command, r git.Runner) error {
	dryRun, _ := cmd.Flags().GetBool(flagDryRun)
	force, _ := cmd.Flags().GetBool(flagForce)
	staleDays, _ := cmd.Flags().GetInt(flagStaleDays)

	mainBranch, err := resolveMainBranch(r)
	if err != nil {
		return err
	}

	hint.New(cmd, hintPainter(cmd)).
		Add(flagDryRun, hintDryRunStale).
		Add(flagForce, hintForce).
		Add(flagStaleDays, "Customize the staleness threshold (default: 14 days)").
		Show()

	s := spinner.New(spinnerOpts(cmd))
	defer s.Stop()

	s.Start("Analyzing worktree activity...")
	candidates, err := findStaleCandidates(r, mainBranch, staleDays)
	if err != nil {
		return err
	}
	s.Stop()

	if len(candidates) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "No stale worktrees found.")
		return nil
	}

	printStaleCandidates(cmd, candidates)

	if dryRun {
		return nil
	}

	if !force && !confirmRemoval(cmd, len(candidates), "stale") {
		fmt.Fprintln(cmd.OutOrStdout(), "Aborted.")
		return nil
	}

	toRemove := make([]cleanCandidate, len(candidates))
	for i, c := range candidates {
		toRemove[i] = c.cleanCandidate
	}

	removed := removeWorktrees(cmd, r, toRemove)
	fmt.Fprintf(cmd.OutOrStdout(), "Cleaned %d stale worktree(s).\n", removed)
	return nil
}

func findStaleCandidates(r git.Runner, mainBranch string, staleDays int) ([]staleCandidate, error) {
	entries, err := git.ListWorktrees(r)
	if err != nil {
		return nil, err
	}

	threshold := time.Now().Add(-time.Duration(staleDays) * 24 * time.Hour)

	var candidates []staleCandidate
	for _, e := range git.FilterEntries(entries, mainBranch) {
		ct, err := git.LastCommitTime(r, e.Branch)
		if err != nil {
			continue
		}

		if ct.Before(threshold) {
			candidates = append(candidates, staleCandidate{
				cleanCandidate: cleanCandidate{path: e.Path, branch: e.Branch},
				lastCommit:     ct,
			})
		}
	}
	return candidates, nil
}

func printStaleCandidates(cmd *cobra.Command, candidates []staleCandidate) {
	prefixes := resolver.AllPrefixes()
	fmt.Fprintln(cmd.OutOrStdout(), "Stale worktrees:")
	for _, c := range candidates {
		task, _ := resolver.TaskFromBranch(c.branch, prefixes)
		age := resolver.FormatAge(c.lastCommit)
		fmt.Fprintf(cmd.OutOrStdout(), "  %s (%s) — last commit: %s\n", task, c.branch, age)
	}
}

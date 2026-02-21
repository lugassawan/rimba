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

	hintMerged       = "Remove worktrees whose branches are already merged into main"
	hintDryRunPrune  = "Preview what would be pruned without making changes"
	hintDryRunMerged = "Preview what would be removed without making changes"
	hintDryRunStale  = "Preview what would be removed without making changes"
	hintForce        = "Skip confirmation prompt"
)

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
	candidates, err := operations.FindMergedCandidates(r, mergeRef, mainBranch)
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

	items := operations.RemoveCandidates(r, candidates, func(msg string) {
		s.Update(msg)
	})
	printCleanedItems(cmd, items)

	removed := countRemoved(items)
	fmt.Fprintf(cmd.OutOrStdout(), "Cleaned %d merged worktree(s).\n", removed)
	return nil
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
	candidates, err := operations.FindStaleCandidates(r, mainBranch, staleDays)
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

	toRemove := make([]operations.CleanCandidate, len(candidates))
	for i, c := range candidates {
		toRemove[i] = c.CleanCandidate
	}

	items := operations.RemoveCandidates(r, toRemove, func(msg string) {
		s.Update(msg)
	})
	printCleanedItems(cmd, items)

	removed := countRemoved(items)
	fmt.Fprintf(cmd.OutOrStdout(), "Cleaned %d stale worktree(s).\n", removed)
	return nil
}

func printMergedCandidates(cmd *cobra.Command, candidates []operations.CleanCandidate) {
	prefixes := resolver.AllPrefixes()
	fmt.Fprintln(cmd.OutOrStdout(), "Merged worktrees:")
	for _, c := range candidates {
		task, _ := resolver.TaskFromBranch(c.Branch, prefixes)
		fmt.Fprintf(cmd.OutOrStdout(), "  %s (%s)\n", task, c.Branch)
	}
}

func printStaleCandidates(cmd *cobra.Command, candidates []operations.StaleCandidate) {
	prefixes := resolver.AllPrefixes()
	fmt.Fprintln(cmd.OutOrStdout(), "Stale worktrees:")
	for _, c := range candidates {
		task, _ := resolver.TaskFromBranch(c.Branch, prefixes)
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

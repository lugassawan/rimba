package cmd

import (
	"bufio"
	"fmt"
	"strings"

	"github.com/lugassawan/rimba/internal/git"
	"github.com/lugassawan/rimba/internal/hint"
	"github.com/lugassawan/rimba/internal/resolver"
	"github.com/lugassawan/rimba/internal/spinner"
	"github.com/spf13/cobra"
)

const (
	flagDryRun = "dry-run"
	flagMerged = "merged"

	hintMerged       = "Remove worktrees whose branches are already merged into main"
	hintDryRunPrune  = "Preview what would be pruned without making changes"
	hintDryRunMerged = "Preview what would be removed without making changes"
	hintForce        = "Skip confirmation prompt"
)

type mergedCandidate struct {
	path   string
	branch string
}

func init() {
	cleanCmd.Flags().Bool(flagDryRun, false, "Show what would be pruned/removed without making changes")
	cleanCmd.Flags().Bool(flagMerged, false, "Remove worktrees whose branches are merged into main")
	cleanCmd.Flags().Bool(flagForce, false, "Skip confirmation prompt when used with --merged")

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

		if merged {
			return cleanMerged(cmd, r)
		}
		return cleanPrune(cmd, r)
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
	s.Start("Fetching from origin...")
	if err := git.Fetch(r, "origin"); err != nil {
		s.Stop()
		fmt.Fprintf(cmd.OutOrStdout(), "Warning: fetch failed (no remote?): continuing with local state\n")
	}

	s.Update("Analyzing branches...")
	candidates, err := findMergedCandidates(r, mainBranch)
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

	if !force && !confirmRemoval(cmd, len(candidates)) {
		fmt.Fprintln(cmd.OutOrStdout(), "Aborted.")
		return nil
	}

	removed := removeMergedWorktrees(cmd, r, candidates)
	fmt.Fprintf(cmd.OutOrStdout(), "Cleaned %d merged worktree(s).\n", removed)
	return nil
}

func findMergedCandidates(r git.Runner, mainBranch string) ([]mergedCandidate, error) {
	mergedList, err := git.MergedBranches(r, mainBranch)
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

	var candidates []mergedCandidate
	for _, e := range entries {
		if e.Branch == "" || e.Branch == mainBranch {
			continue
		}
		if mergedSet[e.Branch] {
			candidates = append(candidates, mergedCandidate{path: e.Path, branch: e.Branch})
		}
	}
	return candidates, nil
}

func printMergedCandidates(cmd *cobra.Command, candidates []mergedCandidate) {
	prefixes := resolver.AllPrefixes()
	fmt.Fprintln(cmd.OutOrStdout(), "Merged worktrees:")
	for _, c := range candidates {
		task, _ := resolver.TaskFromBranch(c.branch, prefixes)
		fmt.Fprintf(cmd.OutOrStdout(), "  %s (%s)\n", task, c.branch)
	}
}

func confirmRemoval(cmd *cobra.Command, count int) bool {
	fmt.Fprintf(cmd.OutOrStdout(), "\nRemove %d merged worktree(s)? [y/N] ", count)
	reader := bufio.NewReader(cmd.InOrStdin())
	answer, _ := reader.ReadString('\n')
	answer = strings.TrimSpace(strings.ToLower(answer))
	return answer == "y" || answer == "yes"
}

func removeMergedWorktrees(cmd *cobra.Command, r git.Runner, candidates []mergedCandidate) int {
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

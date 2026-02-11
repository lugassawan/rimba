package cmd

import (
	"fmt"
	"strings"

	"github.com/lugassawan/rimba/internal/config"
	"github.com/lugassawan/rimba/internal/conflict"
	"github.com/lugassawan/rimba/internal/git"
	"github.com/lugassawan/rimba/internal/hint"
	"github.com/lugassawan/rimba/internal/resolver"
	"github.com/lugassawan/rimba/internal/spinner"
	"github.com/lugassawan/rimba/internal/termcolor"
	"github.com/spf13/cobra"
)

const (
	flagDryMerge = "dry-merge"

	hintDryMerge = "Also check for real merge conflicts (git 2.38+)"
)

func init() {
	conflictCheckCmd.Flags().Bool(flagDryMerge, false, "Also check for real merge conflicts (requires git 2.38+)")
	conflictCheckCmd.ValidArgsFunction = func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return completeWorktreeTasks(cmd, toComplete), cobra.ShellCompDirectiveNoFileComp
	}

	rootCmd.AddCommand(conflictCheckCmd)
}

var conflictCheckCmd = &cobra.Command{
	Use:   "conflict-check [task...]",
	Short: "Detect file overlaps across worktrees",
	Long: `Analyzes active worktree branches for file-level overlaps that may cause merge conflicts.

Examples:
  rimba conflict-check                      # Check all active branches
  rimba conflict-check --dry-merge          # Also do in-memory merge check
  rimba conflict-check auth-task api-task   # Check specific pair`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg := config.FromContext(cmd.Context())
		r := newRunner()
		dryMerge, _ := cmd.Flags().GetBool(flagDryMerge)

		hint.New(cmd, hintPainter(cmd)).
			Add(flagDryMerge, hintDryMerge).
			Show()

		s := spinner.New(spinnerOpts(cmd))
		defer s.Stop()
		s.Start("Analyzing branches...")

		branches, err := collectBranches(r, cfg, args)
		if err != nil {
			return err
		}

		if len(branches) < 2 {
			s.Stop()
			fmt.Fprintln(cmd.OutOrStdout(), "Need at least 2 branches to check for conflicts.")
			return nil
		}

		analysis, err := conflict.Analyze(r, cfg.DefaultSource, branches, dryMerge)
		if err != nil {
			return fmt.Errorf("conflict analysis failed: %w", err)
		}

		s.Stop()
		printConflictAnalysis(cmd, analysis, dryMerge)
		return nil
	},
}

func collectBranches(r git.Runner, cfg *config.Config, tasks []string) ([]string, error) {
	worktrees, err := listWorktreeInfos(r)
	if err != nil {
		return nil, err
	}

	prefixes := resolver.AllPrefixes()

	if len(tasks) > 0 {
		// Resolve specific tasks to branches.
		var branches []string
		for _, task := range tasks {
			wt, found := resolver.FindBranchForTask(task, worktrees, prefixes)
			if !found {
				return nil, fmt.Errorf(errWorktreeNotFound, task)
			}
			branches = append(branches, wt.Branch)
		}
		return branches, nil
	}

	// Collect all non-main branches.
	var branches []string
	for _, wt := range worktrees {
		if wt.Branch == "" || wt.Branch == cfg.DefaultSource {
			continue
		}
		branches = append(branches, wt.Branch)
	}
	return branches, nil
}

func printConflictAnalysis(cmd *cobra.Command, analysis *conflict.Analysis, dryMerge bool) {
	noColor, _ := cmd.Flags().GetBool(flagNoColor)
	p := termcolor.NewPainter(noColor)

	if len(analysis.Overlaps) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), p.Paint("No file overlaps detected.", termcolor.Green))
		return
	}

	fmt.Fprintln(cmd.OutOrStdout(), p.Paint("File overlaps:", termcolor.Bold))
	for _, o := range analysis.Overlaps {
		fmt.Fprintf(cmd.OutOrStdout(), "  %s  ← %s\n",
			p.Paint(o.File, termcolor.Yellow),
			strings.Join(o.Branches, ", "))
	}

	if len(analysis.Pairs) > 0 {
		fmt.Fprintln(cmd.OutOrStdout())
		fmt.Fprintln(cmd.OutOrStdout(), p.Paint("Branch pairs with overlaps:", termcolor.Bold))

		tbl := termcolor.NewTable(2)
		tbl.AddRow(
			p.Paint("BRANCH A", termcolor.Bold),
			p.Paint("BRANCH B", termcolor.Bold),
			p.Paint("FILES", termcolor.Bold),
			conflictHeader(p, dryMerge),
		)

		for _, pair := range analysis.Pairs {
			conflictCell := ""
			if dryMerge {
				if pair.HasConflict {
					conflictCell = p.Paint("CONFLICT", termcolor.Red)
				} else {
					conflictCell = p.Paint("clean", termcolor.Green)
				}
			}
			tbl.AddRow(pair.BranchA, pair.BranchB, fmt.Sprintf("%d", len(pair.OverlapFiles)), conflictCell)
		}

		tbl.Render(cmd.OutOrStdout())
	}

	fmt.Fprintf(cmd.OutOrStdout(), "\n%d file(s) touched by multiple branches, %d pair(s) with overlaps\n",
		len(analysis.Overlaps), len(analysis.Pairs))
}

func conflictHeader(p *termcolor.Painter, dryMerge bool) string {
	if dryMerge {
		return p.Paint("MERGE", termcolor.Bold)
	}
	return ""
}

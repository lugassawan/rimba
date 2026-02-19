package cmd

import (
	"fmt"
	"strings"

	"github.com/lugassawan/rimba/internal/config"
	"github.com/lugassawan/rimba/internal/conflict"
	"github.com/lugassawan/rimba/internal/hint"
	"github.com/lugassawan/rimba/internal/operations"
	"github.com/lugassawan/rimba/internal/resolver"
	"github.com/lugassawan/rimba/internal/spinner"
	"github.com/lugassawan/rimba/internal/termcolor"
	"github.com/spf13/cobra"
)

const (
	flagDryMerge = "dry-merge"

	hintDryMerge = "Simulate merges with git merge-tree (git 2.38+)"
)

func init() {
	conflictCheckCmd.Flags().Bool(flagDryMerge, false, "simulate merges with git merge-tree (git 2.38+)")
	rootCmd.AddCommand(conflictCheckCmd)
}

var conflictCheckCmd = &cobra.Command{
	Use:   "conflict-check",
	Short: "Detect file overlaps between worktree branches",
	Long:  "Scans all active worktrees and reports files modified in multiple branches, indicating potential merge conflicts.",
	RunE: func(cmd *cobra.Command, _ []string) error {
		cfg := config.FromContext(cmd.Context())

		r := newRunner()

		worktrees, err := listWorktreeInfos(r)
		if err != nil {
			return err
		}

		prefixes := resolver.AllPrefixes()
		allTasks := operations.CollectTasks(worktrees, prefixes)
		eligible := operations.FilterEligible(worktrees, prefixes, cfg.DefaultSource, allTasks, true)

		if len(eligible) == 0 {
			fmt.Fprintln(cmd.OutOrStdout(), "No active worktree branches found.")
			return nil
		}

		hint.New(cmd, hintPainter(cmd)).
			Add(flagDryMerge, hintDryMerge).
			Show()

		s := spinner.New(spinnerOpts(cmd))
		defer s.Stop()
		s.Start("Collecting file changes...")

		diffs, err := conflict.CollectDiffs(r, cfg.DefaultSource, eligible)
		if err != nil {
			return err
		}

		result := conflict.DetectOverlaps(diffs)

		dryMerge, _ := cmd.Flags().GetBool(flagDryMerge)
		var dryResults []conflict.DryMergeResult
		if dryMerge {
			s.Update("Running dry merges...")
			dryResults, err = conflict.DryMergeAll(r, eligible)
			if err != nil {
				return err
			}
		}

		s.Stop()

		noColor, _ := cmd.Flags().GetBool(flagNoColor)
		p := termcolor.NewPainter(noColor)

		if len(result.Overlaps) == 0 && !hasConflicts(dryResults) {
			fmt.Fprintln(cmd.OutOrStdout(), "No file overlaps found between active worktree branches.")
			return nil
		}

		if len(result.Overlaps) > 0 {
			renderOverlapTable(cmd, p, result, prefixes)
		}

		if dryMerge {
			renderDryMergeResults(cmd, p, dryResults, prefixes)
		}

		fmt.Fprintf(cmd.OutOrStdout(), "\n%d file overlap(s) found across %d branches.\n",
			len(result.Overlaps), result.TotalBranches)

		return nil
	},
}

func renderOverlapTable(cmd *cobra.Command, p *termcolor.Painter, result *conflict.CheckResult, prefixes []string) {
	tbl := termcolor.NewTable(2)
	tbl.AddRow(
		p.Paint("FILE", termcolor.Bold),
		p.Paint("BRANCHES", termcolor.Bold),
		p.Paint("SEVERITY", termcolor.Bold),
	)

	for _, o := range result.Overlaps {
		branchLabels := make([]string, len(o.Branches))
		for i, b := range o.Branches {
			task, prefix := resolver.TaskFromBranch(b, prefixes)
			if prefix != "" {
				branchLabels[i] = task
			} else {
				branchLabels[i] = b
			}
		}

		sevLabel := conflict.SeverityLabel(o)
		var sevColor termcolor.Color
		if o.Severity == conflict.SeverityHigh {
			sevColor = termcolor.Red
		} else {
			sevColor = termcolor.Yellow
		}

		tbl.AddRow(
			o.File,
			strings.Join(branchLabels, ", "),
			p.Paint(sevLabel, sevColor),
		)
	}

	tbl.Render(cmd.OutOrStdout())
}

func renderDryMergeResults(cmd *cobra.Command, p *termcolor.Painter, results []conflict.DryMergeResult, prefixes []string) {
	var conflicting []conflict.DryMergeResult
	for _, r := range results {
		if r.HasConflicts {
			conflicting = append(conflicting, r)
		}
	}

	if len(conflicting) == 0 {
		return
	}

	fmt.Fprintln(cmd.OutOrStdout())
	fmt.Fprintln(cmd.OutOrStdout(), p.Paint("Dry merge conflicts:", termcolor.Bold))

	tbl := termcolor.NewTable(2)
	tbl.AddRow(
		p.Paint("BRANCH 1", termcolor.Bold),
		p.Paint("BRANCH 2", termcolor.Bold),
		p.Paint("CONFLICT FILES", termcolor.Bold),
	)

	for _, r := range conflicting {
		task1, _ := resolver.TaskFromBranch(r.Branch1, prefixes)
		task2, _ := resolver.TaskFromBranch(r.Branch2, prefixes)
		files := p.Paint(strings.Join(r.ConflictFiles, ", "), termcolor.Red)
		tbl.AddRow(task1, task2, files)
	}

	tbl.Render(cmd.OutOrStdout())
}

func hasConflicts(results []conflict.DryMergeResult) bool {
	for _, r := range results {
		if r.HasConflicts {
			return true
		}
	}
	return false
}

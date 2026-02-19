package cmd

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/lugassawan/rimba/internal/config"
	"github.com/lugassawan/rimba/internal/conflict"
	"github.com/lugassawan/rimba/internal/operations"
	"github.com/lugassawan/rimba/internal/resolver"
	"github.com/lugassawan/rimba/internal/spinner"
	"github.com/lugassawan/rimba/internal/termcolor"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(mergePlanCmd)
}

var mergePlanCmd = &cobra.Command{
	Use:   "merge-plan",
	Short: "Recommend optimal merge order",
	Long:  "Analyzes file overlaps between worktree branches and recommends a merge order that minimizes conflicts.",
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

		s := spinner.New(spinnerOpts(cmd))
		defer s.Stop()
		s.Start("Collecting file changes...")

		diffs, err := conflict.CollectDiffs(r, cfg.DefaultSource, eligible)
		if err != nil {
			return err
		}

		result := conflict.DetectOverlaps(diffs)

		branchNames := make([]string, len(eligible))
		for i, wt := range eligible {
			branchNames[i] = wt.Branch
		}

		steps := conflict.PlanMergeOrder(result.Overlaps, branchNames)

		s.Stop()

		noColor, _ := cmd.Flags().GetBool(flagNoColor)
		p := termcolor.NewPainter(noColor)

		tbl := termcolor.NewTable(2)
		tbl.AddRow(
			p.Paint("ORDER", termcolor.Bold),
			p.Paint("BRANCH", termcolor.Bold),
			p.Paint("CONFLICTS", termcolor.Bold),
		)

		for _, step := range steps {
			task, prefix := resolver.TaskFromBranch(step.Branch, prefixes)
			typeName := strings.TrimSuffix(prefix, "/")

			branchCell := task
			if c := typeColor(typeName); c != "" {
				branchCell = p.Paint(branchCell, c)
			}

			conflictStr := strconv.Itoa(step.Conflicts)
			var conflictColor termcolor.Color
			if step.Conflicts == 0 {
				conflictColor = termcolor.Green
			} else {
				conflictColor = termcolor.Yellow
			}

			tbl.AddRow(
				strconv.Itoa(step.Order),
				branchCell,
				p.Paint(conflictStr, conflictColor),
			)
		}

		tbl.Render(cmd.OutOrStdout())
		fmt.Fprintln(cmd.OutOrStdout(), "\nMerge in this order to minimize conflicts.")

		return nil
	},
}

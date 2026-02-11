package cmd

import (
	"fmt"

	"github.com/lugassawan/rimba/internal/config"
	"github.com/lugassawan/rimba/internal/conflict"
	"github.com/lugassawan/rimba/internal/spinner"
	"github.com/lugassawan/rimba/internal/termcolor"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(mergePlanCmd)
}

var mergePlanCmd = &cobra.Command{
	Use:   "merge-plan",
	Short: "Suggest optimal merge order",
	Long: `Analyzes file overlaps across branches and suggests a merge order that
minimizes conflict risk. Branches with fewer overlaps are recommended first.

Examples:
  rimba merge-plan`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg := config.FromContext(cmd.Context())
		r := newRunner()

		s := spinner.New(spinnerOpts(cmd))
		defer s.Stop()
		s.Start("Computing merge plan...")

		branches, err := collectBranches(r, cfg, nil)
		if err != nil {
			return err
		}

		if len(branches) < 2 {
			s.Stop()
			fmt.Fprintln(cmd.OutOrStdout(), "Need at least 2 branches to plan merge order.")
			return nil
		}

		analysis, err := conflict.Analyze(r, cfg.DefaultSource, branches, false)
		if err != nil {
			return fmt.Errorf("conflict analysis failed: %w", err)
		}

		plan := conflict.Plan(analysis)
		s.Stop()

		printMergePlan(cmd, plan, branches)
		return nil
	},
}

func printMergePlan(cmd *cobra.Command, plan *conflict.MergePlan, allBranches []string) {
	noColor, _ := cmd.Flags().GetBool(flagNoColor)
	p := termcolor.NewPainter(noColor)

	if len(plan.Steps) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), p.Paint("No overlaps detected — branches can be merged in any order.", termcolor.Green))

		if len(allBranches) > 0 {
			for i, b := range allBranches {
				fmt.Fprintf(cmd.OutOrStdout(), "  %d. %s\n", i+1, b)
			}
		}
		return
	}

	fmt.Fprintln(cmd.OutOrStdout(), p.Paint("Recommended merge order:", termcolor.Bold))

	// Print branches with overlaps in order.
	step := 1
	overlappingSet := make(map[string]struct{})
	for _, s := range plan.Steps {
		overlappingSet[s.Branch] = struct{}{}
	}

	// First: branches without any overlaps (safe to merge anytime).
	for _, b := range allBranches {
		if _, hasOverlap := overlappingSet[b]; !hasOverlap {
			fmt.Fprintf(cmd.OutOrStdout(), "  %d. %s %s\n", step,
				p.Paint(b, termcolor.Green),
				p.Paint("(no overlaps)", termcolor.Gray))
			step++
		}
	}

	// Then: branches with overlaps in greedy order.
	for _, s := range plan.Steps {
		label := ""
		if s.OverlapCount > 0 {
			label = p.Paint(fmt.Sprintf("(%d file overlaps)", s.OverlapCount), termcolor.Yellow)
		} else {
			label = p.Paint("(no remaining overlaps)", termcolor.Gray)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "  %d. %s %s\n", step, s.Branch, label)
		step++
	}
}

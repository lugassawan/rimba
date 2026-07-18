package cmd

import (
	"fmt"
	"path/filepath"

	"github.com/lugassawan/rimba/internal/config"
	"github.com/lugassawan/rimba/internal/git"
	"github.com/lugassawan/rimba/internal/metrics"
	"github.com/lugassawan/rimba/internal/output"
	"github.com/spf13/cobra"
)

const (
	flagLast    = "last"
	flagCommand = "command"

	msgNoMetrics = "No metrics collected yet. Run 'rimba add' to start collecting."
)

var reportCmd = &cobra.Command{
	Use:   "report",
	Short: "Show aggregated timing metrics collected by rimba add",
	Long: "Aggregates the timing spans rimba add has recorded to .rimba/metrics.jsonl " +
		"into per-command, per-phase count/p50/p95/mean statistics.",
	Example: `  rimba report
  rimba report --last 20
  rimba report --command add
  rimba report --json`,
	Annotations: map[string]string{"skipConfig": "true"},
	RunE: func(cmd *cobra.Command, args []string) error {
		r := newRunner(cmd.Context())
		repoRoot, err := git.MainRepoRoot(cmd.Context(), r)
		if err != nil {
			return err
		}

		path := filepath.Join(repoRoot, config.DirName, metricsFileName)
		runs, err := metrics.ReadRuns(path)
		if err != nil {
			return err
		}

		commandFilter, _ := cmd.Flags().GetString(flagCommand)
		last, _ := cmd.Flags().GetInt(flagLast)
		reports := metrics.Aggregate(filterRuns(runs, commandFilter, last))

		return renderReport(cmd, reports)
	},
}

// filterRuns applies --command first (keep only runs with a matching
// Command), then trims to the most recent --last N of the already-filtered
// set (last <= 0 means no trim). Order matters for deterministic output:
// filter-then-trim, not trim-then-filter.
func filterRuns(runs []metrics.Run, command string, last int) []metrics.Run {
	if command != "" {
		filtered := make([]metrics.Run, 0, len(runs))
		for _, run := range runs {
			if run.Command == command {
				filtered = append(filtered, run)
			}
		}
		runs = filtered
	}

	if last > 0 && last < len(runs) {
		runs = runs[len(runs)-last:]
	}
	return runs
}

// renderReport writes reports as a JSON envelope (--json) or a plain table.
func renderReport(cmd *cobra.Command, reports []metrics.CommandReport) error {
	if isJSON(cmd) {
		return output.WriteJSON(cmd.OutOrStdout(), version, "report", output.ReportData{
			Commands: reportsToJSON(reports),
		})
	}

	if len(reports) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), msgNoMetrics)
		return nil
	}

	out := cmd.OutOrStdout()
	for _, r := range reports {
		fmt.Fprintf(out, "%s (%d run(s))\n", r.Command, r.Count)
		for _, p := range r.Phases {
			fmt.Fprintf(out, "  %-20s count=%-5d p50=%-6dms p95=%-6dms mean=%-6dms\n",
				p.Name, p.Count, p.P50MS, p.P95MS, p.MeanMS)
		}
	}
	return nil
}

// reportsToJSON converts internal metrics.CommandReport values into their
// JSON mirror type, never returning a nil slice (so JSON mode always emits
// a valid, non-null "commands" array).
func reportsToJSON(reports []metrics.CommandReport) []output.CommandReportJSON {
	items := make([]output.CommandReportJSON, 0, len(reports))
	for _, r := range reports {
		phases := make([]output.PhaseStatJSON, 0, len(r.Phases))
		for _, p := range r.Phases {
			phases = append(phases, output.PhaseStatJSON{
				Name:   p.Name,
				Count:  p.Count,
				P50MS:  p.P50MS,
				P95MS:  p.P95MS,
				MeanMS: p.MeanMS,
			})
		}
		items = append(items, output.CommandReportJSON{
			Command: r.Command,
			Count:   r.Count,
			Phases:  phases,
		})
	}
	return items
}

func init() {
	reportCmd.Flags().Int(flagLast, 0, "only include the most recent N runs (0 = all)")
	reportCmd.Flags().String(flagCommand, "", "filter to runs matching this command name")
	rootCmd.AddCommand(reportCmd)
}

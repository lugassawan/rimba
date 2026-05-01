package cmd

import (
	"context"
	"fmt"
	"io"
	"strconv"
	"time"

	"github.com/lugassawan/rimba/internal/operations"
	"github.com/lugassawan/rimba/internal/output"
	"github.com/lugassawan/rimba/internal/resolver"
	"github.com/lugassawan/rimba/internal/spinner"
	"github.com/lugassawan/rimba/internal/termcolor"
	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:         "status",
	Short:       "Show worktree dashboard with summary stats and age info",
	Long:        "Displays a summary of all worktrees including total count, dirty, stale, and behind counts, plus per-worktree age information.",
	Annotations: map[string]string{"skipConfig": "true"},
	RunE: func(cmd *cobra.Command, args []string) error {
		r := newRunner()
		staleDays, _ := cmd.Flags().GetInt(flagStaleDays)
		detail, _ := cmd.Flags().GetBool(flagDetail)

		s := spinner.New(spinnerOpts(cmd))
		s.Start("Collecting worktree status...")
		res, err := operations.StatusDashboard(context.Background(), r, operations.StatusDashboardRequest{Detail: detail})
		s.Stop()
		if err != nil {
			return err
		}

		if len(res.Entries) == 0 {
			if isJSON(cmd) {
				return output.WriteJSON(cmd.OutOrStdout(), version, "status", output.StatusData{
					Summary:   output.StatusSummary{},
					Worktrees: make([]output.StatusItem, 0),
					StaleDays: staleDays,
				})
			}
			fmt.Fprintln(cmd.OutOrStdout(), "No worktrees found.")
			return nil
		}

		if isJSON(cmd) {
			return writeStatusJSON(cmd, res.Entries, staleDays, res.Footprint)
		}

		noColor, _ := cmd.Flags().GetBool(flagNoColor)
		p := termcolor.NewPainter(noColor)
		renderStatusDashboard(cmd.OutOrStdout(), p, statusRender{
			results:   res.Entries,
			staleDays: staleDays,
			detail:    detail,
			footprint: res.Footprint,
		})
		return nil
	},
}

func init() {
	statusCmd.Flags().Int(flagStaleDays, defaultStaleDays, "Number of days after which a worktree is considered stale")
	statusCmd.Flags().Bool(flagDetail, false, "Show per-worktree disk size and 7-day commit velocity")
	rootCmd.AddCommand(statusCmd)
}

type statusRender struct {
	results   []operations.StatusEntry
	staleDays int
	detail    bool
	footprint *operations.DiskFootprint
}

// renderStatusDashboard prints the summary header and per-worktree table.
func renderStatusDashboard(out io.Writer, p *termcolor.Painter, r statusRender) {
	staleThreshold := time.Now().Add(-time.Duration(r.staleDays) * 24 * time.Hour)
	summary := operations.SummarizeStatus(r.results, staleThreshold)
	prefixes := resolver.AllPrefixes()

	fmt.Fprintf(out, "Worktrees: %s  Dirty: %s  Stale: %s  Behind: %s\n",
		p.Paint(strconv.Itoa(summary.Total), termcolor.Bold),
		colorCount(p, summary.Dirty, termcolor.Yellow),
		colorCount(p, summary.Stale, termcolor.Red),
		colorCount(p, summary.Behind, termcolor.Red),
	)
	if r.detail && r.footprint != nil {
		fmt.Fprintln(out, formatDiskLine(p, r.footprint))
	}
	fmt.Fprintln(out)

	tbl := termcolor.NewTable(2)
	header := []string{
		p.Paint("TASK", termcolor.Bold),
		p.Paint("TYPE", termcolor.Bold),
		p.Paint("BRANCH", termcolor.Bold),
		p.Paint("STATUS", termcolor.Bold),
		p.Paint("AGE", termcolor.Bold),
	}
	if r.detail {
		header = append(header,
			p.Paint("SIZE", termcolor.Bold),
			p.Paint("7D", termcolor.Bold),
		)
	}
	tbl.AddRow(header...)

	for _, e := range r.results {
		tbl.AddRow(buildStatusRow(e, prefixes, staleThreshold, p, r.detail)...)
	}

	tbl.Render(out)
	renderActionHints(out, p, summary)
}

// formatDiskLine builds the "Disk: total X  (main: Y, worktrees: Z)"
// summary. The main: fragment is dropped when main-size errored.
func formatDiskLine(p *termcolor.Painter, fp *operations.DiskFootprint) string {
	total := p.Paint(resolver.FormatBytes(fp.TotalBytes), termcolor.Bold)
	worktrees := resolver.FormatBytes(fp.WorktreesBytes)
	if fp.MainErr != nil {
		return fmt.Sprintf("Disk: total %s  (worktrees: %s)", total, worktrees)
	}
	main := resolver.FormatBytes(fp.MainBytes)
	return fmt.Sprintf("Disk: total %s  (main: %s, worktrees: %s)", total, main, worktrees)
}

// renderActionHints prints one-line next-step suggestions derived from the
// summary counts. Emits nothing when all counts are zero. The hints are
// CLI-only and never appear in the JSON output.
func renderActionHints(out io.Writer, p *termcolor.Painter, summary operations.StatusSummary) {
	if summary.Behind == 0 && summary.Stale == 0 && summary.Dirty == 0 {
		return
	}

	fmt.Fprintln(out)

	if summary.Behind > 0 {
		fmt.Fprintf(out, "%s %d behind main. Run: %s\n",
			p.Paint("→", termcolor.Yellow),
			summary.Behind,
			p.Paint("rimba sync --all", termcolor.Bold),
		)
	}
	if summary.Stale > 0 {
		fmt.Fprintf(out, "%s %d stale. Run: %s\n",
			p.Paint("→", termcolor.Yellow),
			summary.Stale,
			p.Paint("rimba clean --stale", termcolor.Bold),
		)
	}
	if summary.Dirty > 0 {
		fmt.Fprintf(out, "%s %d dirty. Review uncommitted changes before merging.\n",
			p.Paint("→", termcolor.Yellow),
			summary.Dirty,
		)
	}
}

// buildStatusRow formats a single worktree row for the status table.
func buildStatusRow(r operations.StatusEntry, prefixes []string, staleThreshold time.Time, p *termcolor.Painter, detail bool) []string {
	task, typeName := resolver.TaskAndType(r.Entry.Branch, prefixes)

	taskCell := "  " + task
	typeCell := typeName
	if c := typeColor(typeName); c != "" {
		typeCell = p.Paint(typeCell, c)
	}

	row := []string{taskCell, typeCell, r.Entry.Branch, colorStatus(p, r.Status), formatAgeCell(r, staleThreshold, p)}
	if detail {
		row = append(row, formatSizeCell(r, p), formatRecentCell(r, p))
	}
	return row
}

// formatSizeCell renders the SIZE column. Nil (errored) renders as "?".
func formatSizeCell(r operations.StatusEntry, p *termcolor.Painter) string {
	if r.SizeBytes == nil {
		return p.Paint("?", termcolor.Gray)
	}
	return resolver.FormatBytes(*r.SizeBytes)
}

// formatRecentCell renders the 7D column. Nil (errored) renders as "?".
func formatRecentCell(r operations.StatusEntry, p *termcolor.Painter) string {
	if r.Recent7D == nil {
		return p.Paint("?", termcolor.Gray)
	}
	return strconv.Itoa(*r.Recent7D)
}

// formatAgeCell formats the age cell with color and stale indicator.
func formatAgeCell(r operations.StatusEntry, staleThreshold time.Time, p *termcolor.Painter) string {
	if !r.HasTime {
		return p.Paint("unknown", termcolor.Gray)
	}
	ageStr := resolver.FormatAge(r.CommitTime)
	if r.CommitTime.Before(staleThreshold) {
		return p.Paint(ageStr, termcolor.Red) + " " + p.Paint("⚠ stale", termcolor.Red)
	}
	return p.Paint(ageStr, resolver.AgeColor(r.CommitTime))
}

// writeStatusJSON builds the JSON output for the status command.
func writeStatusJSON(cmd *cobra.Command, results []operations.StatusEntry, staleDays int, footprint *operations.DiskFootprint) error {
	staleThreshold := time.Now().Add(-time.Duration(staleDays) * 24 * time.Hour)
	prefixes := resolver.AllPrefixes()

	opsSummary := operations.SummarizeStatus(results, staleThreshold)
	summary := output.StatusSummary{
		Total:  opsSummary.Total,
		Dirty:  opsSummary.Dirty,
		Stale:  opsSummary.Stale,
		Behind: opsSummary.Behind,
	}

	items := make([]output.StatusItem, 0, len(results))
	for _, r := range results {
		task, typeName := resolver.TaskAndType(r.Entry.Branch, prefixes)

		item := output.StatusItem{
			Task:      task,
			Type:      typeName,
			Branch:    r.Entry.Branch,
			Status:    r.Status,
			SizeBytes: r.SizeBytes,
			Recent7D:  r.Recent7D,
		}

		if r.HasTime {
			stale := r.CommitTime.Before(staleThreshold)
			item.Age = &output.StatusAge{
				LastCommit: r.CommitTime.UTC().Format(time.RFC3339),
				Stale:      stale,
			}
		}

		items = append(items, item)
	}

	data := output.StatusData{
		Summary:   summary,
		Worktrees: items,
		StaleDays: staleDays,
	}
	if footprint != nil {
		disk := &output.DiskSummary{
			TotalBytes:     footprint.TotalBytes,
			WorktreesBytes: footprint.WorktreesBytes,
		}
		if footprint.MainErr == nil {
			main := footprint.MainBytes
			disk.MainBytes = &main
		}
		data.Disk = disk
	}
	return output.WriteJSON(cmd.OutOrStdout(), version, "status", data)
}

// colorCount formats a count with color if non-zero.
func colorCount(p *termcolor.Painter, count int, color termcolor.Color) string {
	s := strconv.Itoa(count)
	if count > 0 {
		return p.Paint(s, color, termcolor.Bold)
	}
	return s
}

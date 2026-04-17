package cmd

import (
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/lugassawan/rimba/internal/fsutil"
	"github.com/lugassawan/rimba/internal/git"
	"github.com/lugassawan/rimba/internal/operations"
	"github.com/lugassawan/rimba/internal/output"
	"github.com/lugassawan/rimba/internal/parallel"
	"github.com/lugassawan/rimba/internal/resolver"
	"github.com/lugassawan/rimba/internal/spinner"
	"github.com/lugassawan/rimba/internal/termcolor"
	"github.com/spf13/cobra"
)

const recentWindow = 7 * 24 * time.Hour

type statusEntry struct {
	entry      git.WorktreeEntry
	status     resolver.WorktreeStatus
	commitTime time.Time
	hasTime    bool
	sizeBytes  *int64
	recent7D   *int
}

var statusCmd = &cobra.Command{
	Use:         "status",
	Short:       "Show worktree dashboard with summary stats and age info",
	Long:        "Displays a summary of all worktrees including total count, dirty, stale, and behind counts, plus per-worktree age information.",
	Annotations: map[string]string{"skipConfig": "true"},
	RunE: func(cmd *cobra.Command, args []string) error {
		r := newRunner()

		mainBranch, err := resolveMainBranch(r)
		if err != nil {
			return err
		}

		entries, err := git.ListWorktrees(r)
		if err != nil {
			return err
		}

		mainEntry := git.FindEntry(entries, mainBranch)
		candidates := git.FilterEntries(entries, mainBranch)

		staleDays, _ := cmd.Flags().GetInt(flagStaleDays)
		detail, _ := cmd.Flags().GetBool(flagDetail)

		if len(candidates) == 0 {
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

		s := spinner.New(spinnerOpts(cmd))
		defer s.Stop()
		s.Start("Collecting worktree status...")

		var mainSize int64
		var mainErr error
		var mainWG sync.WaitGroup
		if detail && mainEntry != nil {
			mainWG.Add(1)
			go func(path string) {
				defer mainWG.Done()
				mainSize, mainErr = fsutil.DirSize(path)
			}(mainEntry.Path)
		}

		results := collectStatuses(r, candidates, s, detail)
		mainWG.Wait()
		s.Stop()

		var footprint *operations.DiskFootprint
		if detail {
			sizes := make([]*int64, len(results))
			for i, r := range results {
				sizes[i] = r.sizeBytes
			}
			fp := operations.BuildDiskFootprint(sizes, mainSize, mainErr)
			footprint = &fp
			sortBySizeDesc(results)
		}

		if isJSON(cmd) {
			return writeStatusJSON(cmd, results, staleDays, footprint)
		}

		noColor, _ := cmd.Flags().GetBool(flagNoColor)
		p := termcolor.NewPainter(noColor)

		renderStatusDashboard(cmd.OutOrStdout(), p, results, staleDays, detail, footprint)
		return nil
	},
}

func init() {
	statusCmd.Flags().Int(flagStaleDays, defaultStaleDays, "Number of days after which a worktree is considered stale")
	statusCmd.Flags().Bool(flagDetail, false, "Show per-worktree disk size and 7-day commit velocity")
	rootCmd.AddCommand(statusCmd)
}

// collectStatuses gathers dirty/ahead/behind state and last commit time
// per candidate. Under detail it also computes size and 7-day velocity;
// per-item errors leave the pointer nil (non-fatal).
func collectStatuses(r git.Runner, candidates []git.WorktreeEntry, s *spinner.Spinner, detail bool) []statusEntry {
	s.Update("Collecting status...")
	return parallel.Collect(len(candidates), 8, func(i int) statusEntry {
		e := candidates[i]
		st := operations.CollectWorktreeStatus(r, e.Path)
		var ct time.Time
		var hasTime bool
		if t, err := git.LastCommitTime(r, e.Branch); err == nil {
			ct = t
			hasTime = true
		}
		se := statusEntry{entry: e, status: st, commitTime: ct, hasTime: hasTime}
		if detail {
			if n, err := fsutil.DirSize(e.Path); err == nil {
				se.sizeBytes = &n
			}
			if c, err := git.CommitCountSince(r, e.Branch, recentWindow); err == nil {
				se.recent7D = &c
			}
		}
		return se
	})
}

// sortBySizeDesc sorts largest first, stable, nils last.
func sortBySizeDesc(results []statusEntry) {
	sort.SliceStable(results, func(i, j int) bool {
		a, b := results[i].sizeBytes, results[j].sizeBytes
		switch {
		case a == nil && b == nil:
			return false
		case a == nil:
			return false
		case b == nil:
			return true
		default:
			return *a > *b
		}
	})
}

// renderStatusDashboard prints the summary header and per-worktree table.
func renderStatusDashboard(out io.Writer, p *termcolor.Painter, results []statusEntry, staleDays int, detail bool, footprint *operations.DiskFootprint) {
	staleThreshold := time.Now().Add(-time.Duration(staleDays) * 24 * time.Hour)
	summary := buildCLIStatusSummary(results, staleThreshold)
	prefixes := resolver.AllPrefixes()

	fmt.Fprintf(out, "Worktrees: %s  Dirty: %s  Stale: %s  Behind: %s\n",
		p.Paint(strconv.Itoa(summary.total), termcolor.Bold),
		colorCount(p, summary.dirty, termcolor.Yellow),
		colorCount(p, summary.stale, termcolor.Red),
		colorCount(p, summary.behind, termcolor.Red),
	)
	if detail && footprint != nil {
		fmt.Fprintln(out, formatDiskLine(p, footprint))
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
	if detail {
		header = append(header,
			p.Paint("SIZE", termcolor.Bold),
			p.Paint("7D", termcolor.Bold),
		)
	}
	tbl.AddRow(header...)

	for _, r := range results {
		tbl.AddRow(buildStatusRow(r, prefixes, staleThreshold, p, detail)...)
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
func renderActionHints(out io.Writer, p *termcolor.Painter, summary cliStatusSummary) {
	if summary.behind == 0 && summary.stale == 0 && summary.dirty == 0 {
		return
	}

	fmt.Fprintln(out)

	if summary.behind > 0 {
		fmt.Fprintf(out, "%s %d behind main. Run: %s\n",
			p.Paint("→", termcolor.Yellow),
			summary.behind,
			p.Paint("rimba sync --all", termcolor.Bold),
		)
	}
	if summary.stale > 0 {
		fmt.Fprintf(out, "%s %d stale. Run: %s\n",
			p.Paint("→", termcolor.Yellow),
			summary.stale,
			p.Paint("rimba clean --stale", termcolor.Bold),
		)
	}
	if summary.dirty > 0 {
		fmt.Fprintf(out, "%s %d dirty. Review uncommitted changes before merging.\n",
			p.Paint("→", termcolor.Yellow),
			summary.dirty,
		)
	}
}

type cliStatusSummary struct {
	total, dirty, stale, behind int
}

// buildCLIStatusSummary counts summary stats from results.
func buildCLIStatusSummary(results []statusEntry, staleThreshold time.Time) cliStatusSummary {
	s := cliStatusSummary{total: len(results)}
	for _, r := range results {
		if r.status.Dirty {
			s.dirty++
		}
		if r.status.Behind > 0 {
			s.behind++
		}
		if r.hasTime && r.commitTime.Before(staleThreshold) {
			s.stale++
		}
	}
	return s
}

// buildStatusRow formats a single worktree row for the status table.
func buildStatusRow(r statusEntry, prefixes []string, staleThreshold time.Time, p *termcolor.Painter, detail bool) []string {
	task, matchedPrefix := resolver.PureTaskFromBranch(r.entry.Branch, prefixes)
	typeName := strings.TrimSuffix(matchedPrefix, "/")

	taskCell := "  " + task
	typeCell := typeName
	if c := typeColor(typeName); c != "" {
		typeCell = p.Paint(typeCell, c)
	}

	row := []string{taskCell, typeCell, r.entry.Branch, colorStatus(p, r.status), formatAgeCell(r, staleThreshold, p)}
	if detail {
		row = append(row, formatSizeCell(r, p), formatRecentCell(r, p))
	}
	return row
}

// formatSizeCell renders the SIZE column. Nil (errored) renders as "?".
func formatSizeCell(r statusEntry, p *termcolor.Painter) string {
	if r.sizeBytes == nil {
		return p.Paint("?", termcolor.Gray)
	}
	return resolver.FormatBytes(*r.sizeBytes)
}

// formatRecentCell renders the 7D column. Nil (errored) renders as "?".
func formatRecentCell(r statusEntry, p *termcolor.Painter) string {
	if r.recent7D == nil {
		return p.Paint("?", termcolor.Gray)
	}
	return strconv.Itoa(*r.recent7D)
}

// formatAgeCell formats the age cell with color and stale indicator.
func formatAgeCell(r statusEntry, staleThreshold time.Time, p *termcolor.Painter) string {
	if !r.hasTime {
		return p.Paint("unknown", termcolor.Gray)
	}
	ageStr := resolver.FormatAge(r.commitTime)
	if r.commitTime.Before(staleThreshold) {
		return p.Paint(ageStr, termcolor.Red) + " " + p.Paint("⚠ stale", termcolor.Red)
	}
	return p.Paint(ageStr, resolver.AgeColor(r.commitTime))
}

// writeStatusJSON builds the JSON output for the status command.
func writeStatusJSON(cmd *cobra.Command, results []statusEntry, staleDays int, footprint *operations.DiskFootprint) error {
	staleThreshold := time.Now().Add(-time.Duration(staleDays) * 24 * time.Hour)
	prefixes := resolver.AllPrefixes()

	var summary output.StatusSummary
	summary.Total = len(results)

	items := make([]output.StatusItem, 0, len(results))
	for _, r := range results {
		if r.status.Dirty {
			summary.Dirty++
		}
		if r.status.Behind > 0 {
			summary.Behind++
		}

		task, matchedPrefix := resolver.PureTaskFromBranch(r.entry.Branch, prefixes)
		typeName := strings.TrimSuffix(matchedPrefix, "/")

		item := output.StatusItem{
			Task:      task,
			Type:      typeName,
			Branch:    r.entry.Branch,
			Status:    r.status,
			SizeBytes: r.sizeBytes,
			Recent7D:  r.recent7D,
		}

		if r.hasTime {
			stale := r.commitTime.Before(staleThreshold)
			if stale {
				summary.Stale++
			}
			item.Age = &output.StatusAge{
				LastCommit: r.commitTime.UTC().Format(time.RFC3339),
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

package cmd

import (
	"fmt"
	"io"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/lugassawan/rimba/internal/git"
	"github.com/lugassawan/rimba/internal/operations"
	"github.com/lugassawan/rimba/internal/output"
	"github.com/lugassawan/rimba/internal/resolver"
	"github.com/lugassawan/rimba/internal/spinner"
	"github.com/lugassawan/rimba/internal/termcolor"
	"github.com/spf13/cobra"
)

type statusJSONData struct {
	Summary   statusJSONSummary `json:"summary"`
	Worktrees []statusJSONItem  `json:"worktrees"`
	StaleDays int               `json:"stale_days"`
}

type statusJSONSummary struct {
	Total  int `json:"total"`
	Dirty  int `json:"dirty"`
	Stale  int `json:"stale"`
	Behind int `json:"behind"`
}

type statusJSONItem struct {
	Task   string                  `json:"task"`
	Type   string                  `json:"type"`
	Branch string                  `json:"branch"`
	Status resolver.WorktreeStatus `json:"status"`
	Age    *statusJSONAge          `json:"age"`
}

type statusJSONAge struct {
	LastCommit string `json:"last_commit"`
	Stale      bool   `json:"stale"`
}

type statusEntry struct {
	entry      git.WorktreeEntry
	status     resolver.WorktreeStatus
	commitTime time.Time
	hasTime    bool
}

func init() {
	statusCmd.Flags().Int(flagStaleDays, defaultStaleDays, "Number of days after which a worktree is considered stale")
	rootCmd.AddCommand(statusCmd)
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

		candidates := git.FilterEntries(entries, mainBranch)

		staleDays, _ := cmd.Flags().GetInt(flagStaleDays)

		if len(candidates) == 0 {
			if isJSON(cmd) {
				return output.WriteJSON(cmd.OutOrStdout(), version, "status", statusJSONData{
					Summary:   statusJSONSummary{},
					Worktrees: make([]statusJSONItem, 0),
					StaleDays: staleDays,
				})
			}
			fmt.Fprintln(cmd.OutOrStdout(), "No worktrees found.")
			return nil
		}

		s := spinner.New(spinnerOpts(cmd))
		defer s.Stop()
		s.Start("Collecting worktree status...")

		results := collectStatuses(r, candidates, s)
		s.Stop()

		if isJSON(cmd) {
			return writeStatusJSON(cmd, results, staleDays)
		}

		noColor, _ := cmd.Flags().GetBool(flagNoColor)
		p := termcolor.NewPainter(noColor)

		renderStatusDashboard(cmd.OutOrStdout(), p, results, staleDays)
		return nil
	},
}

// collectStatuses gathers dirty/ahead/behind state and last commit time for each candidate.
func collectStatuses(r git.Runner, candidates []git.WorktreeEntry, s *spinner.Spinner) []statusEntry {
	results := make([]statusEntry, len(candidates))
	var wg sync.WaitGroup
	sem := make(chan struct{}, 8)

	for i, c := range candidates {
		s.Update(fmt.Sprintf("Collecting status... (%d/%d)", i+1, len(candidates)))
		wg.Add(1)
		go func(idx int, e git.WorktreeEntry) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			st := operations.CollectWorktreeStatus(r, e.Path)
			var ct time.Time
			var hasTime bool
			if t, err := git.LastCommitTime(r, e.Branch); err == nil {
				ct = t
				hasTime = true
			}
			results[idx] = statusEntry{entry: e, status: st, commitTime: ct, hasTime: hasTime}
		}(i, c)
	}
	wg.Wait()
	return results
}

// renderStatusDashboard prints the summary header and per-worktree table.
func renderStatusDashboard(out io.Writer, p *termcolor.Painter, results []statusEntry, staleDays int) {
	staleThreshold := time.Now().Add(-time.Duration(staleDays) * 24 * time.Hour)
	summary := buildCLIStatusSummary(results, staleThreshold)
	prefixes := resolver.AllPrefixes()

	fmt.Fprintf(out, "Worktrees: %s  Dirty: %s  Stale: %s  Behind: %s\n\n",
		p.Paint(strconv.Itoa(summary.total), termcolor.Bold),
		colorCount(p, summary.dirty, termcolor.Yellow),
		colorCount(p, summary.stale, termcolor.Red),
		colorCount(p, summary.behind, termcolor.Red),
	)

	tbl := termcolor.NewTable(2)
	tbl.AddRow(
		p.Paint("TASK", termcolor.Bold),
		p.Paint("TYPE", termcolor.Bold),
		p.Paint("BRANCH", termcolor.Bold),
		p.Paint("STATUS", termcolor.Bold),
		p.Paint("AGE", termcolor.Bold),
	)

	for _, r := range results {
		tbl.AddRow(buildStatusRow(r, prefixes, staleThreshold, p)...)
	}

	tbl.Render(out)
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
func buildStatusRow(r statusEntry, prefixes []string, staleThreshold time.Time, p *termcolor.Painter) []string {
	task, matchedPrefix := resolver.TaskFromBranch(r.entry.Branch, prefixes)
	typeName := strings.TrimSuffix(matchedPrefix, "/")

	taskCell := "  " + task
	typeCell := typeName
	if c := typeColor(typeName); c != "" {
		typeCell = p.Paint(typeCell, c)
	}

	return []string{taskCell, typeCell, r.entry.Branch, colorStatus(p, r.status), formatAgeCell(r, staleThreshold, p)}
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
func writeStatusJSON(cmd *cobra.Command, results []statusEntry, staleDays int) error {
	staleThreshold := time.Now().Add(-time.Duration(staleDays) * 24 * time.Hour)
	prefixes := resolver.AllPrefixes()

	var summary statusJSONSummary
	summary.Total = len(results)

	items := make([]statusJSONItem, 0, len(results))
	for _, r := range results {
		if r.status.Dirty {
			summary.Dirty++
		}
		if r.status.Behind > 0 {
			summary.Behind++
		}

		task, matchedPrefix := resolver.TaskFromBranch(r.entry.Branch, prefixes)
		typeName := strings.TrimSuffix(matchedPrefix, "/")

		item := statusJSONItem{
			Task:   task,
			Type:   typeName,
			Branch: r.entry.Branch,
			Status: r.status,
		}

		if r.hasTime {
			stale := r.commitTime.Before(staleThreshold)
			if stale {
				summary.Stale++
			}
			item.Age = &statusJSONAge{
				LastCommit: r.commitTime.UTC().Format(time.RFC3339),
				Stale:      stale,
			}
		}

		items = append(items, item)
	}

	data := statusJSONData{
		Summary:   summary,
		Worktrees: items,
		StaleDays: staleDays,
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

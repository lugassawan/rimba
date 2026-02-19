package cmd

import (
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/lugassawan/rimba/internal/git"
	"github.com/lugassawan/rimba/internal/operations"
	"github.com/lugassawan/rimba/internal/resolver"
	"github.com/lugassawan/rimba/internal/spinner"
	"github.com/lugassawan/rimba/internal/termcolor"
	"github.com/spf13/cobra"
)

const (
	flagStaleDays    = "stale-days"
	defaultStaleDays = 14
)

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

		// Filter out bare entries and main worktree
		var candidates []git.WorktreeEntry
		for _, e := range entries {
			if e.Bare || e.Branch == mainBranch {
				continue
			}
			candidates = append(candidates, e)
		}

		if len(candidates) == 0 {
			fmt.Fprintln(cmd.OutOrStdout(), "No worktrees found.")
			return nil
		}

		staleDays, _ := cmd.Flags().GetInt(flagStaleDays)

		s := spinner.New(spinnerOpts(cmd))
		defer s.Stop()
		s.Start("Collecting worktree status...")

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
		s.Stop()

		// Compute summary
		var totalCount, dirtyCount, staleCount, behindCount int
		totalCount = len(results)
		staleThreshold := time.Now().Add(-time.Duration(staleDays) * 24 * time.Hour)

		for _, r := range results {
			if r.status.Dirty {
				dirtyCount++
			}
			if r.status.Behind > 0 {
				behindCount++
			}
			if r.hasTime && r.commitTime.Before(staleThreshold) {
				staleCount++
			}
		}

		noColor, _ := cmd.Flags().GetBool(flagNoColor)
		p := termcolor.NewPainter(noColor)
		out := cmd.OutOrStdout()
		prefixes := resolver.AllPrefixes()

		// Print header
		fmt.Fprintf(out, "Worktrees: %s  Dirty: %s  Stale: %s  Behind: %s\n\n",
			p.Paint(strconv.Itoa(totalCount), termcolor.Bold),
			colorCount(p, dirtyCount, termcolor.Yellow),
			colorCount(p, staleCount, termcolor.Red),
			colorCount(p, behindCount, termcolor.Red),
		)

		// Print table
		tbl := termcolor.NewTable(2)
		tbl.AddRow(
			p.Paint("TASK", termcolor.Bold),
			p.Paint("TYPE", termcolor.Bold),
			p.Paint("BRANCH", termcolor.Bold),
			p.Paint("STATUS", termcolor.Bold),
			p.Paint("AGE", termcolor.Bold),
		)

		for _, r := range results {
			task, matchedPrefix := resolver.TaskFromBranch(r.entry.Branch, prefixes)
			typeName := strings.TrimSuffix(matchedPrefix, "/")

			taskCell := "  " + task
			typeCell := typeName
			if c := typeColor(typeName); c != "" {
				typeCell = p.Paint(typeCell, c)
			}

			statusCell := colorStatus(p, r.status)

			var ageCell string
			if r.hasTime {
				ageStr := resolver.FormatAge(r.commitTime)
				if r.commitTime.Before(staleThreshold) {
					ageCell = p.Paint(ageStr, termcolor.Red) + " " + p.Paint("⚠ stale", termcolor.Red)
				} else {
					ageCell = p.Paint(ageStr, ageColor(r.commitTime))
				}
			} else {
				ageCell = p.Paint("unknown", termcolor.Gray)
			}

			tbl.AddRow(taskCell, typeCell, r.entry.Branch, statusCell, ageCell)
		}

		tbl.Render(out)
		return nil
	},
}

// colorCount formats a count with color if non-zero.
func colorCount(p *termcolor.Painter, count int, color termcolor.Color) string {
	s := strconv.Itoa(count)
	if count > 0 {
		return p.Paint(s, color, termcolor.Bold)
	}
	return s
}

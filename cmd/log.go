package cmd

import (
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/lugassawan/rimba/internal/git"
	"github.com/lugassawan/rimba/internal/parallel"
	"github.com/lugassawan/rimba/internal/resolver"
	"github.com/lugassawan/rimba/internal/spinner"
	"github.com/lugassawan/rimba/internal/termcolor"
	"github.com/spf13/cobra"
)

const (
	flagLimit = "limit"
	flagSince = "since"
)

type logEntry struct {
	branch     string
	task       string
	typeName   string
	commitTime time.Time
	subject    string
	valid      bool
}

func init() {
	logCmd.Flags().Int(flagLimit, 0, "Maximum number of entries to show (0 = all)")
	logCmd.Flags().String(flagSince, "", "Show entries since duration (e.g. 7d, 2w, 3h)")
	rootCmd.AddCommand(logCmd)
}

var logCmd = &cobra.Command{
	Use:         "log",
	Short:       "Show last commit from each worktree, sorted by recency",
	Long:        "Displays the most recent commit from each worktree, sorted from newest to oldest.",
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

		if len(candidates) == 0 {
			fmt.Fprintln(cmd.OutOrStdout(), "No worktrees found.")
			return nil
		}

		s := spinner.New(spinnerOpts(cmd))
		defer s.Stop()
		s.Start("Collecting commit info...")

		valid := collectLogEntries(r, candidates, s)
		s.Stop()

		// Apply --since filter
		sinceStr, _ := cmd.Flags().GetString(flagSince)
		if sinceStr != "" {
			d, err := resolver.ParseDuration(sinceStr)
			if err != nil {
				return fmt.Errorf("invalid --since value %q: %w", sinceStr, err)
			}
			cutoff := time.Now().Add(-d)
			var filtered []logEntry
			for _, e := range valid {
				if e.commitTime.After(cutoff) {
					filtered = append(filtered, e)
				}
			}
			valid = filtered
		}

		// Apply --limit
		limit, _ := cmd.Flags().GetInt(flagLimit)
		if limit > 0 && len(valid) > limit {
			valid = valid[:limit]
		}

		if len(valid) == 0 {
			fmt.Fprintln(cmd.OutOrStdout(), "No recent commits found.")
			return nil
		}

		noColor, _ := cmd.Flags().GetBool(flagNoColor)
		p := termcolor.NewPainter(noColor)

		renderLogTable(cmd.OutOrStdout(), p, valid)
		return nil
	},
}

// collectLogEntries gathers commit info for each candidate in parallel,
// returning only valid entries sorted by commit time descending.
func collectLogEntries(r git.Runner, candidates []git.WorktreeEntry, s *spinner.Spinner) []logEntry {
	prefixes := resolver.AllPrefixes()
	s.Update("Collecting commit info...")
	results := parallel.Collect(len(candidates), 8, func(i int) logEntry {
		e := candidates[i]
		task, matchedPrefix := resolver.TaskFromBranch(e.Branch, prefixes)
		typeName := strings.TrimSuffix(matchedPrefix, "/")

		ct, subject, err := git.LastCommitInfo(r, e.Branch)
		if err != nil {
			return logEntry{branch: e.Branch, task: task, typeName: typeName}
		}
		return logEntry{
			branch:     e.Branch,
			task:       task,
			typeName:   typeName,
			commitTime: ct,
			subject:    subject,
			valid:      true,
		}
	})

	var valid []logEntry
	for _, r := range results {
		if r.valid {
			valid = append(valid, r)
		}
	}

	sort.Slice(valid, func(i, j int) bool {
		return valid[i].commitTime.After(valid[j].commitTime)
	})

	return valid
}

// renderLogTable prints the log entries as a formatted table.
func renderLogTable(out io.Writer, p *termcolor.Painter, entries []logEntry) {
	fmt.Fprintf(out, "Recent commits across %d worktree(s):\n\n", len(entries))

	tbl := termcolor.NewTable(2)
	tbl.AddRow(
		p.Paint("TASK", termcolor.Bold),
		p.Paint("TYPE", termcolor.Bold),
		p.Paint("AGE", termcolor.Bold),
		p.Paint("COMMIT", termcolor.Bold),
	)

	for _, e := range entries {
		typeCell := e.typeName
		if c := typeColor(e.typeName); c != "" {
			typeCell = p.Paint(typeCell, c)
		}

		ageStr := resolver.FormatAge(e.commitTime)
		ageCell := p.Paint(ageStr, resolver.AgeColor(e.commitTime))

		tbl.AddRow("  "+e.task, typeCell, ageCell, e.subject)
	}

	tbl.Render(out)
}

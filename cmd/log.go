package cmd

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/lugassawan/rimba/internal/git"
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

		var candidates []git.WorktreeEntry
		for _, e := range entries {
			if e.Bare || e.Branch == "" || e.Branch == mainBranch {
				continue
			}
			candidates = append(candidates, e)
		}

		if len(candidates) == 0 {
			fmt.Fprintln(cmd.OutOrStdout(), "No worktrees found.")
			return nil
		}

		s := spinner.New(spinnerOpts(cmd))
		defer s.Stop()
		s.Start("Collecting commit info...")

		results := make([]logEntry, len(candidates))
		prefixes := resolver.AllPrefixes()
		var wg sync.WaitGroup
		sem := make(chan struct{}, 8)

		for i, c := range candidates {
			s.Update(fmt.Sprintf("Collecting commit info... (%d/%d)", i+1, len(candidates)))
			wg.Add(1)
			go func(idx int, e git.WorktreeEntry) {
				defer wg.Done()
				sem <- struct{}{}
				defer func() { <-sem }()

				task, matchedPrefix := resolver.TaskFromBranch(e.Branch, prefixes)
				typeName := strings.TrimSuffix(matchedPrefix, "/")

				ct, subject, err := git.LastCommitInfo(r, e.Branch)
				if err != nil {
					results[idx] = logEntry{branch: e.Branch, task: task, typeName: typeName}
					return
				}
				results[idx] = logEntry{
					branch:     e.Branch,
					task:       task,
					typeName:   typeName,
					commitTime: ct,
					subject:    subject,
					valid:      true,
				}
			}(i, c)
		}
		wg.Wait()
		s.Stop()

		// Collect valid entries and sort by commit time descending
		var valid []logEntry
		for _, r := range results {
			if r.valid {
				valid = append(valid, r)
			}
		}

		sort.Slice(valid, func(i, j int) bool {
			return valid[i].commitTime.After(valid[j].commitTime)
		})

		// Apply --since filter
		sinceStr, _ := cmd.Flags().GetString(flagSince)
		if sinceStr != "" {
			d, err := parseDuration(sinceStr)
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
		out := cmd.OutOrStdout()

		fmt.Fprintf(out, "Recent commits across %d worktree(s):\n\n", len(valid))

		tbl := termcolor.NewTable(2)
		tbl.AddRow(
			p.Paint("TASK", termcolor.Bold),
			p.Paint("TYPE", termcolor.Bold),
			p.Paint("AGE", termcolor.Bold),
			p.Paint("COMMIT", termcolor.Bold),
		)

		for _, e := range valid {
			typeCell := e.typeName
			if c := typeColor(e.typeName); c != "" {
				typeCell = p.Paint(typeCell, c)
			}

			ageStr := resolver.FormatAge(e.commitTime)
			ageCell := p.Paint(ageStr, ageColor(e.commitTime))

			tbl.AddRow("  "+e.task, typeCell, ageCell, e.subject)
		}

		tbl.Render(out)
		return nil
	},
}

// parseDuration parses human-friendly duration strings like "7d", "2w", "3h".
func parseDuration(s string) (time.Duration, error) {
	if len(s) < 2 {
		return 0, fmt.Errorf("duration too short: %q", s)
	}

	numStr := s[:len(s)-1]
	unit := s[len(s)-1]

	n, err := strconv.Atoi(numStr)
	if err != nil {
		return 0, fmt.Errorf("invalid number in duration %q: %w", s, err)
	}
	if n <= 0 {
		return 0, fmt.Errorf("duration must be positive: %q", s)
	}

	switch unit {
	case 'h':
		return time.Duration(n) * time.Hour, nil
	case 'd':
		return time.Duration(n) * 24 * time.Hour, nil
	case 'w':
		return time.Duration(n) * 7 * 24 * time.Hour, nil
	default:
		return 0, fmt.Errorf("unknown duration unit %q in %q (use h, d, or w)", string(unit), s)
	}
}

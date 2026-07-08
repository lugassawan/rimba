package cmd

import (
	"context"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/lugassawan/rimba/internal/config"
	"github.com/lugassawan/rimba/internal/git"
	"github.com/lugassawan/rimba/internal/output"
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
	service    string
	typeName   string
	path       string
	commitTime time.Time
	subject    string
	valid      bool
}

var logCmd = &cobra.Command{
	Use:   "log",
	Short: "Show last commit from each worktree, sorted by recency",
	Long:  "Displays the most recent commit from each worktree, sorted from newest to oldest.",
	Example: `  rimba log
  rimba log --since 7d --limit 10`,
	Annotations: map[string]string{"skipConfig": "true"},
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SetContext(withBestEffortConfig(cmd))
		ctx := cmd.Context()
		r := newRunner(cmd.Context())

		mainBranch, err := resolveMainBranch(ctx, r)
		if err != nil {
			return err
		}

		entries, err := git.ListWorktrees(ctx, r)
		if err != nil {
			return err
		}

		candidates := git.FilterEntries(entries, mainBranch)

		if len(candidates) == 0 {
			if isJSON(cmd) {
				return writeLogJSONEmpty(cmd)
			}
			fmt.Fprintln(cmd.OutOrStdout(), "No worktrees found.")
			return nil
		}

		s := spinner.New(spinnerOpts(cmd))
		defer s.Stop()
		s.Start("Collecting commit info...")

		valid := collectLogEntries(ctx, r, candidates, s)
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
			if isJSON(cmd) {
				return writeLogJSONEmpty(cmd)
			}
			fmt.Fprintln(cmd.OutOrStdout(), "No recent commits found.")
			return nil
		}

		if isJSON(cmd) {
			return writeLogJSON(cmd, valid)
		}

		noColor, _ := cmd.Flags().GetBool(flagNoColor)
		p := termcolor.NewPainter(noColor)

		renderLogTable(cmd.OutOrStdout(), p, valid)
		return nil
	},
}

func init() {
	logCmd.Flags().Int(flagLimit, 0, "maximum number of entries to show (0 = all)")
	logCmd.Flags().String(flagSince, "", "show entries since duration (e.g. 7d, 2w, 3h)")
	rootCmd.AddCommand(logCmd)
}

// collectLogEntries gathers commit info for each candidate in parallel,
// returning only valid entries sorted by commit time descending.
func collectLogEntries(ctx context.Context, r git.Runner, candidates []git.WorktreeEntry, s *spinner.Spinner) []logEntry {
	prefixes := config.PrefixSetFromContext(ctx).Strip()
	s.Update("Collecting commit info...")
	results := parallel.Collect(ctx, len(candidates), 8, func(ctx context.Context, i int) logEntry {
		itemCtx, cancel := git.WithItemTimeout(ctx)
		defer cancel()
		e := candidates[i]
		svc, task, matchedPrefix := resolver.ServiceFromBranch(e.Branch, prefixes)
		typeName := strings.TrimSuffix(matchedPrefix, "/")

		ct, subject, err := git.LastCommitInfo(itemCtx, r, e.Branch)
		if err != nil {
			return logEntry{branch: e.Branch, task: task, service: svc, typeName: typeName, path: e.Path}
		}
		return logEntry{
			branch:     e.Branch,
			task:       task,
			service:    svc,
			typeName:   typeName,
			path:       e.Path,
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

func logEntriesHaveService(entries []logEntry) bool {
	for _, e := range entries {
		if e.service != "" {
			return true
		}
	}
	return false
}

func renderLogTable(out io.Writer, p *termcolor.Painter, entries []logEntry) {
	fmt.Fprintf(out, "Recent commits across %d worktree(s):\n\n", len(entries))

	hasService := logEntriesHaveService(entries)

	tbl := termcolor.NewTable(2)
	header := []string{p.Paint("TASK", termcolor.Bold)}
	if hasService {
		header = append(header, p.Paint("SERVICE", termcolor.Bold))
	}
	header = append(header,
		p.Paint("TYPE", termcolor.Bold),
		p.Paint("AGE", termcolor.Bold),
		p.Paint("COMMIT", termcolor.Bold),
	)
	tbl.AddRow(header...)

	for _, e := range entries {
		typeCell := e.typeName
		if c := typeColor(e.typeName); c != "" {
			typeCell = p.Paint(typeCell, c)
		}

		ageStr := resolver.FormatAge(e.commitTime)
		ageCell := p.Paint(ageStr, resolver.AgeColor(e.commitTime))

		cells := []string{"  " + e.task}
		if hasService {
			cells = append(cells, e.service)
		}
		cells = append(cells, typeCell, ageCell, e.subject)
		tbl.AddRow(cells...)
	}

	tbl.Render(out)
}

func writeLogJSONEmpty(cmd *cobra.Command) error {
	return output.WriteJSON(cmd.OutOrStdout(), version, "log", make([]output.LogItem, 0))
}

func writeLogJSON(cmd *cobra.Command, entries []logEntry) error {
	items := make([]output.LogItem, 0, len(entries))
	for _, e := range entries {
		items = append(items, output.LogItem{
			Task:       e.task,
			Service:    e.service,
			Type:       e.typeName,
			Branch:     e.branch,
			Path:       e.path,
			LastCommit: e.commitTime.UTC().Format(time.RFC3339),
			Subject:    e.subject,
		})
	}
	return output.WriteJSON(cmd.OutOrStdout(), version, "log", items)
}

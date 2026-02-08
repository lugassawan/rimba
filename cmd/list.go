package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/lugassawan/rimba/internal/config"
	"github.com/lugassawan/rimba/internal/git"
	"github.com/lugassawan/rimba/internal/resolver"
	"github.com/lugassawan/rimba/internal/termcolor"
	"github.com/spf13/cobra"
)

var (
	listType   string
	listDirty  bool
	listBehind bool
)

func init() {
	rootCmd.AddCommand(listCmd)

	listCmd.Flags().StringVar(&listType, "type", "", "filter by prefix type (e.g. feature, bugfix)")
	listCmd.Flags().BoolVar(&listDirty, "dirty", false, "show only dirty worktrees")
	listCmd.Flags().BoolVar(&listBehind, "behind", false, "show only worktrees behind upstream")

	_ = listCmd.RegisterFlagCompletionFunc("type", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		var types []string
		for _, p := range resolver.AllPrefixes() {
			t := strings.TrimSuffix(p, "/")
			if strings.HasPrefix(t, toComplete) {
				types = append(types, t)
			}
		}
		return types, cobra.ShellCompDirectiveNoFileComp
	})
}

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List all worktrees",
	Long:  "Lists all git worktrees with their branch, path, and status (dirty, ahead/behind).",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg := config.FromContext(cmd.Context())
		if cfg == nil {
			return errNoConfig
		}

		if listType != "" && !resolver.ValidPrefixType(listType) {
			valid := make([]string, 0, len(resolver.AllPrefixes()))
			for _, p := range resolver.AllPrefixes() {
				valid = append(valid, strings.TrimSuffix(p, "/"))
			}
			return fmt.Errorf("invalid type %q; valid types: %s", listType, strings.Join(valid, ", "))
		}

		r := &git.ExecRunner{}

		repoRoot, err := git.RepoRoot(r)
		if err != nil {
			return err
		}

		entries, err := git.ListWorktrees(r)
		if err != nil {
			return err
		}

		if len(entries) == 0 {
			fmt.Fprintln(cmd.OutOrStdout(), "No worktrees found.")
			return nil
		}

		wtDir := filepath.Join(repoRoot, cfg.WorktreeDir)
		prefixes := resolver.AllPrefixes()

		// Detect current worktree
		cwd, _ := os.Getwd()
		cwdResolved, _ := filepath.EvalSymlinks(cwd)
		cwdResolved = filepath.Clean(cwdResolved)

		var rows []resolver.WorktreeDetail
		for _, e := range entries {
			if e.Bare {
				continue
			}

			// Determine relative path for display
			displayPath := e.Path
			if rel, err := filepath.Rel(wtDir, e.Path); err == nil && len(rel) < len(displayPath) {
				displayPath = rel
			}

			// Build structured status
			var status resolver.WorktreeStatus
			if dirty, err := git.IsDirty(r, e.Path); err == nil && dirty {
				status.Dirty = true
			}
			ahead, behind, _ := git.AheadBehind(r, e.Path)
			status.Ahead = ahead
			status.Behind = behind

			// Detect if this is the current worktree
			entryResolved, _ := filepath.EvalSymlinks(e.Path)
			entryResolved = filepath.Clean(entryResolved)
			isCurrent := cwdResolved == entryResolved

			rows = append(rows, resolver.NewWorktreeDetail(e.Branch, prefixes, displayPath, status, isCurrent))
		}

		filtered := rows[:0]
		for _, row := range rows {
			if listType != "" && row.Type != listType {
				continue
			}
			if listDirty && !row.Status.Dirty {
				continue
			}
			if listBehind && row.Status.Behind == 0 {
				continue
			}
			filtered = append(filtered, row)
		}
		rows = filtered

		if len(rows) == 0 {
			fmt.Fprintln(cmd.OutOrStdout(), "No worktrees match the given filters.")
			return nil
		}

		resolver.SortDetailsByTask(rows)

		// Setup color painter
		noColor, _ := cmd.Flags().GetBool("no-color")
		p := termcolor.NewPainter(noColor)

		tbl := termcolor.NewTable(2)
		tbl.AddRow(
			p.Paint("TASK", termcolor.Bold),
			p.Paint("TYPE", termcolor.Bold),
			p.Paint("BRANCH", termcolor.Bold),
			p.Paint("PATH", termcolor.Bold),
			p.Paint("STATUS", termcolor.Bold),
		)

		for _, row := range rows {
			taskCell := "  " + row.Task
			if row.IsCurrent {
				taskCell = "* " + row.Task
				taskCell = p.Paint(taskCell, termcolor.Green, termcolor.Bold)
			}

			typeCell := row.Type
			if c := typeColor(row.Type); c != "" {
				typeCell = p.Paint(typeCell, c)
			}

			statusCell := colorStatus(p, row.Status)

			tbl.AddRow(taskCell, typeCell, row.Branch, row.Path, statusCell)
		}

		tbl.Render(cmd.OutOrStdout())
		return nil
	},
}

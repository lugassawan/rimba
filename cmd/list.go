package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/lugassawan/rimba/internal/config"
	"github.com/lugassawan/rimba/internal/git"
	"github.com/lugassawan/rimba/internal/hint"
	"github.com/lugassawan/rimba/internal/operations"
	"github.com/lugassawan/rimba/internal/resolver"
	"github.com/lugassawan/rimba/internal/spinner"
	"github.com/lugassawan/rimba/internal/termcolor"
	"github.com/spf13/cobra"
)

const (
	flagType     = "type"
	flagDirty    = "dirty"
	flagBehind   = "behind"
	flagArchived = "archived"

	hintType   = "Filter by prefix type (feature, bugfix, hotfix, etc.)"
	hintDirty  = "Show only worktrees with uncommitted changes"
	hintBehind = "Show only worktrees behind upstream"
)

// candidate holds a pre-filtered worktree entry before status collection.
type candidate struct {
	entry       git.WorktreeEntry
	displayPath string
	isCurrent   bool
}

var (
	listType     string
	listDirty    bool
	listBehind   bool
	listArchived bool
)

func init() {
	rootCmd.AddCommand(listCmd)

	listCmd.Flags().StringVar(&listType, flagType, "", "filter by prefix type (e.g. feature, bugfix)")
	listCmd.Flags().BoolVar(&listDirty, flagDirty, false, "show only dirty worktrees")
	listCmd.Flags().BoolVar(&listBehind, flagBehind, false, "show only worktrees behind upstream")
	listCmd.Flags().BoolVar(&listArchived, flagArchived, false, "show archived branches (not in any active worktree)")

	listCmd.MarkFlagsMutuallyExclusive(flagArchived, flagType)
	listCmd.MarkFlagsMutuallyExclusive(flagArchived, flagDirty)
	listCmd.MarkFlagsMutuallyExclusive(flagArchived, flagBehind)

	_ = listCmd.RegisterFlagCompletionFunc(flagType, func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
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
		if listArchived {
			r := newRunner()
			mainBranch, err := resolveMainBranch(r)
			if err != nil {
				return err
			}
			return listArchivedBranches(cmd, r, mainBranch)
		}

		cfg := config.FromContext(cmd.Context())

		if listType != "" && !resolver.ValidPrefixType(listType) {
			valid := make([]string, 0, len(resolver.AllPrefixes()))
			for _, p := range resolver.AllPrefixes() {
				valid = append(valid, strings.TrimSuffix(p, "/"))
			}
			return fmt.Errorf("invalid type %q; valid types: %s", listType, strings.Join(valid, ", "))
		}

		r := newRunner()

		repoRoot, err := git.MainRepoRoot(r)
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

		hint.New(cmd, hintPainter(cmd)).
			Add(flagType, hintType).
			Add(flagDirty, hintDirty).
			Add(flagBehind, hintBehind).
			Show()

		s := spinner.New(spinnerOpts(cmd))
		defer s.Stop()
		s.Start("Loading worktrees...")

		var candidates []candidate
		for _, e := range entries {
			if e.Bare {
				continue
			}

			if listType != "" {
				_, matchedPrefix := resolver.TaskFromBranch(e.Branch, prefixes)
				entryType := strings.TrimSuffix(matchedPrefix, "/")
				if entryType != listType {
					continue
				}
			}

			displayPath := e.Path
			if rel, err := filepath.Rel(wtDir, e.Path); err == nil && len(rel) < len(displayPath) {
				displayPath = rel
			}

			entryResolved, _ := filepath.EvalSymlinks(e.Path)
			entryResolved = filepath.Clean(entryResolved)
			isCurrent := cwdResolved == entryResolved

			candidates = append(candidates, candidate{entry: e, displayPath: displayPath, isCurrent: isCurrent})
		}

		rows := make([]resolver.WorktreeDetail, len(candidates))
		var wg sync.WaitGroup
		sem := make(chan struct{}, 8)

		for i, c := range candidates {
			s.Update(fmt.Sprintf("Loading worktrees... (%d/%d)", i+1, len(candidates)))
			wg.Add(1)
			go func(idx int, c candidate) {
				defer wg.Done()
				sem <- struct{}{}
				defer func() { <-sem }()

				status := operations.CollectWorktreeStatus(r, c.entry.Path)
				rows[idx] = resolver.NewWorktreeDetail(c.entry.Branch, prefixes, c.displayPath, status, c.isCurrent)
			}(i, c)
		}
		wg.Wait()

		rows = operations.FilterDetailsByStatus(rows, listDirty, listBehind)

		s.Stop()

		if len(rows) == 0 {
			fmt.Fprintln(cmd.OutOrStdout(), "No worktrees match the given filters.")
			return nil
		}

		resolver.SortDetailsByTask(rows)

		// Setup color painter
		noColor, _ := cmd.Flags().GetBool(flagNoColor)
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

// listArchivedBranches shows branches not associated with any active worktree.
func listArchivedBranches(cmd *cobra.Command, r git.Runner, mainBranch string) error {
	archived, err := operations.ListArchivedBranches(r, mainBranch)
	if err != nil {
		return err
	}

	if len(archived) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "No archived branches found.")
		return nil
	}

	noColor, _ := cmd.Flags().GetBool(flagNoColor)
	p := termcolor.NewPainter(noColor)
	prefixes := resolver.AllPrefixes()

	tbl := termcolor.NewTable(2)
	tbl.AddRow(
		p.Paint("TASK", termcolor.Bold),
		p.Paint("TYPE", termcolor.Bold),
		p.Paint("BRANCH", termcolor.Bold),
	)

	for _, b := range archived {
		task, matchedPrefix := resolver.TaskFromBranch(b, prefixes)
		typeName := strings.TrimSuffix(matchedPrefix, "/")

		typeCell := typeName
		if c := typeColor(typeName); c != "" {
			typeCell = p.Paint(typeCell, c)
		}

		tbl.AddRow("  "+task, typeCell, b)
	}

	fmt.Fprintln(cmd.OutOrStdout(), "Archived branches:")
	tbl.Render(cmd.OutOrStdout())
	fmt.Fprintf(cmd.OutOrStdout(), "\nTo restore: rimba restore <task>\n")
	return nil
}

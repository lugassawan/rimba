package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/lugassawan/rimba/internal/config"
	"github.com/lugassawan/rimba/internal/gh"
	"github.com/lugassawan/rimba/internal/git"
	"github.com/lugassawan/rimba/internal/hint"
	"github.com/lugassawan/rimba/internal/operations"
	"github.com/lugassawan/rimba/internal/output"
	"github.com/lugassawan/rimba/internal/parallel"
	"github.com/lugassawan/rimba/internal/resolver"
	"github.com/lugassawan/rimba/internal/spinner"
	"github.com/lugassawan/rimba/internal/termcolor"
	"github.com/spf13/cobra"
)

const prQueryTimeout = 10 * time.Second

// prInfo is the per-branch PR/CI summary. Nil fields mean unknown.
type prInfo struct {
	number *int
	status *string
}

// prInfoMap is keyed by branch. A nil map means gh was not queried.
type prInfoMap map[string]prInfo

type collectPair struct {
	detail resolver.WorktreeDetail
	info   prInfo
}

const (
	flagType     = "type"
	flagDirty    = "dirty"
	flagBehind   = "behind"
	flagArchived = "archived"
	flagFull     = "full"
	flagService  = "service"

	hintType    = "Filter by prefix type (feature, bugfix, hotfix, etc.)"
	hintDirty   = "Show only worktrees with uncommitted changes"
	hintBehind  = "Show only worktrees behind upstream"
	hintFull    = "Show all columns (branch, path, PR/CI when gh is available)"
	hintService = "Filter by service name (monorepo)"
)

// candidate holds a pre-filtered worktree entry before status collection.
type candidate struct {
	entry       git.WorktreeEntry
	displayPath string
	isCurrent   bool
}

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List all worktrees",
	Long:  "Lists all git worktrees with task, type, and status. Use --full to show branch, path, and (when gh is installed and authenticated) PR number and CI rollup.",
	RunE: func(cmd *cobra.Command, args []string) error {
		opts := listReadFlags(cmd)

		if opts.archived {
			r := newRunner()
			mainBranch, err := resolveMainBranch(r)
			if err != nil {
				return err
			}
			return listArchivedBranches(cmd, r, mainBranch)
		}

		if err := listValidateType(opts.typeFilter); err != nil {
			return err
		}

		r := newRunner()
		entries, wtDir, err := listLoadEntries(r, cmd)
		if err != nil {
			return err
		}

		if len(entries) == 0 {
			return listRenderEmpty(cmd, "No worktrees found.")
		}

		listShowHints(cmd)

		s := spinner.New(spinnerOpts(cmd))
		defer s.Stop()
		s.Start("Loading worktrees...")

		prefixes := resolver.AllPrefixes()
		candidates := listBuildCandidates(entries, wtDir, prefixes, opts.typeFilter)
		rows, prInfos, ghWarning := listCollectAll(cmd.Context(), r, candidates, prefixes, opts.full)
		rows = operations.FilterDetailsByStatus(rows, opts.dirty, opts.behind)
		rows = resolver.FilterByService(rows, opts.service)

		s.Stop()

		if len(rows) == 0 {
			return listRenderEmpty(cmd, "No worktrees match the given filters.")
		}

		resolver.SortDetailsByTask(rows)

		if isJSON(cmd) {
			return listRenderJSON(cmd, rows, prInfos)
		}
		listRenderTable(cmd, rows, opts.full, prInfos, ghWarning)
		return nil
	},
}

// listOpts holds parsed list flags.
type listOpts struct {
	typeFilter string
	dirty      bool
	behind     bool
	archived   bool
	full       bool
	service    string
}

func listReadFlags(cmd *cobra.Command) listOpts {
	typeFilter, _ := cmd.Flags().GetString(flagType)
	dirty, _ := cmd.Flags().GetBool(flagDirty)
	behind, _ := cmd.Flags().GetBool(flagBehind)
	archived, _ := cmd.Flags().GetBool(flagArchived)
	full, _ := cmd.Flags().GetBool(flagFull)
	service, _ := cmd.Flags().GetString(flagService)
	return listOpts{
		typeFilter: typeFilter,
		dirty:      dirty,
		behind:     behind,
		archived:   archived,
		full:       full,
		service:    service,
	}
}

func listValidateType(typeFilter string) error {
	if typeFilter == "" || resolver.ValidPrefixType(typeFilter) {
		return nil
	}
	valid := make([]string, 0, len(resolver.AllPrefixes()))
	for _, p := range resolver.AllPrefixes() {
		valid = append(valid, strings.TrimSuffix(p, "/"))
	}
	return fmt.Errorf("invalid type %q; valid types: %s", typeFilter, strings.Join(valid, ", "))
}

func listLoadEntries(r git.Runner, cmd *cobra.Command) ([]git.WorktreeEntry, string, error) {
	cfg := config.FromContext(cmd.Context())
	repoRoot, err := git.MainRepoRoot(r)
	if err != nil {
		return nil, "", err
	}
	entries, err := git.ListWorktrees(r)
	if err != nil {
		return nil, "", err
	}
	return entries, filepath.Join(repoRoot, cfg.WorktreeDir), nil
}

func listShowHints(cmd *cobra.Command) {
	if isJSON(cmd) {
		return
	}
	hint.New(cmd, hintPainter(cmd)).
		Add(flagFull, hintFull).
		Add(flagType, hintType).
		Add(flagService, hintService).
		Add(flagDirty, hintDirty).
		Add(flagBehind, hintBehind).
		Show()
}

func listRenderEmpty(cmd *cobra.Command, msg string) error {
	if isJSON(cmd) {
		return output.WriteJSON(cmd.OutOrStdout(), version, "list", make([]output.ListItem, 0))
	}
	fmt.Fprintln(cmd.OutOrStdout(), msg)
	return nil
}

func listBuildCandidates(entries []git.WorktreeEntry, wtDir string, prefixes []string, typeFilter string) []candidate {
	cwd, _ := os.Getwd()
	cwdResolved, _ := filepath.EvalSymlinks(cwd)
	cwdResolved = filepath.Clean(cwdResolved)

	var candidates []candidate
	for _, e := range entries {
		if e.Bare {
			continue
		}

		if typeFilter != "" {
			_, matchedPrefix := resolver.TaskFromBranch(e.Branch, prefixes)
			entryType := strings.TrimSuffix(matchedPrefix, "/")
			if entryType != typeFilter {
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
	return candidates
}

// listCollectAll gathers worktree details and, under --full, open PR/CI
// rollup per branch in parallel. ghWarning is set when --full is requested
// but gh is missing or unauthenticated.
func listCollectAll(ctx context.Context, r git.Runner, candidates []candidate, prefixes []string, full bool) (rows []resolver.WorktreeDetail, prInfos prInfoMap, ghWarning string) {
	var ghRunner gh.Runner
	if full {
		ghRunner = gh.Default()
		if err := gh.CheckAuth(ctx, ghRunner); err != nil {
			ghWarning = "gh unavailable; PR/CI columns blank"
			ghRunner = nil
		} else {
			prInfos = make(prInfoMap, len(candidates))
		}
	}

	results := parallel.Collect(len(candidates), 8, func(i int) collectPair {
		c := candidates[i]
		status := operations.CollectWorktreeStatus(r, c.entry.Path)
		d := resolver.NewWorktreeDetail(c.entry.Branch, prefixes, c.displayPath, status, c.isCurrent)
		var info prInfo
		if ghRunner != nil {
			info = queryPRInfo(ctx, ghRunner, c.entry.Branch)
		}
		return collectPair{detail: d, info: info}
	})

	rows = make([]resolver.WorktreeDetail, len(results))
	for i, res := range results {
		rows[i] = res.detail
		if prInfos != nil {
			prInfos[res.detail.Branch] = res.info
		}
	}
	return rows, prInfos, ghWarning
}

// queryPRInfo runs one gh pr list under a timeout. Errors degrade silently
// so one slow or broken query does not fail the whole table.
func queryPRInfo(ctx context.Context, ghRunner gh.Runner, branch string) prInfo {
	qctx, cancel := context.WithTimeout(ctx, prQueryTimeout)
	defer cancel()
	pr, err := gh.QueryPRStatus(qctx, ghRunner, branch)
	if err != nil || pr.Number == 0 {
		return prInfo{}
	}
	n := pr.Number
	info := prInfo{number: &n}
	if pr.CIStatus != "" {
		s := pr.CIStatus
		info.status = &s
	}
	return info
}

func listRenderJSON(cmd *cobra.Command, rows []resolver.WorktreeDetail, prInfos prInfoMap) error {
	items := make([]output.ListItem, len(rows))
	for i, r := range rows {
		items[i] = output.ListItem{
			Task:      r.Task,
			Service:   r.Service,
			Type:      r.Type,
			Branch:    r.Branch,
			Path:      r.Path,
			IsCurrent: r.IsCurrent,
			Status:    r.Status,
		}
		if info, ok := prInfos[r.Branch]; ok {
			items[i].PRNumber = info.number
			items[i].CIStatus = info.status
		}
	}
	return output.WriteJSON(cmd.OutOrStdout(), version, "list", items)
}

func listRenderTable(cmd *cobra.Command, rows []resolver.WorktreeDetail, full bool, prInfos prInfoMap, ghWarning string) {
	hasService := resolver.HasService(rows)
	noColor, _ := cmd.Flags().GetBool(flagNoColor)
	p := termcolor.NewPainter(noColor)

	if ghWarning != "" {
		fmt.Fprintln(cmd.OutOrStdout(), p.Paint(ghWarning, termcolor.Yellow))
	}

	tbl := termcolor.NewTable(2)
	tbl.AddRow(listHeader(p, hasService, full)...)

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
		cells := listRow(taskCell, row, typeCell, statusCell, hasService, full)
		if full {
			info := prInfos[row.Branch]
			cells = append(cells, formatPRCell(info.number, p), formatCICell(info.status, p))
		}
		tbl.AddRow(cells...)
	}

	tbl.Render(cmd.OutOrStdout())
}

func init() {
	rootCmd.AddCommand(listCmd)

	listCmd.Flags().String(flagType, "", "filter by prefix type (e.g. feature, bugfix)")
	listCmd.Flags().String(flagService, "", "filter by service name (monorepo)")
	listCmd.Flags().Bool(flagDirty, false, "show only dirty worktrees")
	listCmd.Flags().Bool(flagBehind, false, "show only worktrees behind upstream")
	listCmd.Flags().Bool(flagArchived, false, "show archived branches (not in any active worktree)")
	listCmd.Flags().Bool(flagFull, false, "show all columns (branch, path, PR/CI when gh is available)")

	listCmd.MarkFlagsMutuallyExclusive(flagArchived, flagType)
	listCmd.MarkFlagsMutuallyExclusive(flagArchived, flagDirty)
	listCmd.MarkFlagsMutuallyExclusive(flagArchived, flagBehind)
	listCmd.MarkFlagsMutuallyExclusive(flagArchived, flagFull)

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

func listHeader(p *termcolor.Painter, hasService, full bool) []string {
	h := []string{p.Paint("TASK", termcolor.Bold)}
	if hasService {
		h = append(h, p.Paint("SERVICE", termcolor.Bold))
	}
	h = append(h, p.Paint("TYPE", termcolor.Bold))
	if full {
		h = append(h, p.Paint("BRANCH", termcolor.Bold), p.Paint("PATH", termcolor.Bold))
	}
	h = append(h, p.Paint("STATUS", termcolor.Bold))
	if full {
		h = append(h, p.Paint("PR", termcolor.Bold), p.Paint("CI", termcolor.Bold))
	}
	return h
}

func listRow(taskCell string, row resolver.WorktreeDetail, typeCell, statusCell string, hasService, full bool) []string {
	cells := []string{taskCell}
	if hasService {
		cells = append(cells, row.Service)
	}
	cells = append(cells, typeCell)
	if full {
		cells = append(cells, row.Branch, row.Path)
	}
	cells = append(cells, statusCell)
	return cells
}

func formatPRCell(n *int, p *termcolor.Painter) string {
	if n == nil {
		return p.Paint("–", termcolor.Gray)
	}
	return fmt.Sprintf("#%d", *n)
}

func formatCICell(status *string, p *termcolor.Painter) string {
	if status == nil {
		return p.Paint("–", termcolor.Gray)
	}
	switch *status {
	case "SUCCESS":
		return p.Paint("✓", termcolor.Green)
	case "PENDING":
		return p.Paint("●", termcolor.Yellow)
	case "FAILURE":
		return p.Paint("✗", termcolor.Red)
	default:
		return p.Paint("–", termcolor.Gray)
	}
}

// listArchivedBranches shows branches not associated with any active worktree.

func listArchivedBranches(cmd *cobra.Command, r git.Runner, mainBranch string) error {
	archived, err := operations.ListArchivedBranches(r, mainBranch)
	if err != nil {
		return err
	}

	prefixes := resolver.AllPrefixes()

	if isJSON(cmd) {
		items := make([]output.ListArchivedItem, 0, len(archived))
		for _, b := range archived {
			task, matchedPrefix := resolver.PureTaskFromBranch(b, prefixes)
			typeName := strings.TrimSuffix(matchedPrefix, "/")
			items = append(items, output.ListArchivedItem{Task: task, Type: typeName, Branch: b})
		}
		return output.WriteJSON(cmd.OutOrStdout(), version, "list", items)
	}

	if len(archived) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "No archived branches found.")
		return nil
	}

	noColor, _ := cmd.Flags().GetBool(flagNoColor)
	p := termcolor.NewPainter(noColor)

	tbl := termcolor.NewTable(2)
	tbl.AddRow(
		p.Paint("TASK", termcolor.Bold),
		p.Paint("TYPE", termcolor.Bold),
		p.Paint("BRANCH", termcolor.Bold),
	)

	for _, b := range archived {
		task, matchedPrefix := resolver.PureTaskFromBranch(b, prefixes)
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

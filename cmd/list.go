package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/lugassawan/rimba/internal/config"
	"github.com/lugassawan/rimba/internal/gh"
	"github.com/lugassawan/rimba/internal/git"
	"github.com/lugassawan/rimba/internal/hint"
	"github.com/lugassawan/rimba/internal/operations"
	"github.com/lugassawan/rimba/internal/output"
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
	flagFull     = "full"
	flagService  = "service"

	hintType    = "Filter by prefix type (feature, bugfix, hotfix, etc.)"
	hintDirty   = "Show only worktrees with uncommitted changes"
	hintBehind  = "Show only worktrees behind upstream"
	hintFull    = "Show all columns (branch, path, PR/CI when gh is available)"
	hintService = "Filter by service name (monorepo)"
)

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
		cfg := config.FromContext(cmd.Context())
		repoRoot, err := git.MainRepoRoot(r)
		if err != nil {
			return err
		}
		cwd, _ := os.Getwd()

		var ghR gh.Runner
		if opts.full {
			ghR = gh.Default()
		}

		listShowHints(cmd)

		s := spinner.New(spinnerOpts(cmd))
		defer s.Stop()
		s.Start("Loading worktrees...")

		res, err := operations.ListWorktrees(cmd.Context(), r, ghR, operations.ListWorktreesRequest{
			Full:        opts.full,
			TypeFilter:  opts.typeFilter,
			Dirty:       opts.dirty,
			Behind:      opts.behind,
			Service:     opts.service,
			CurrentPath: cwd,
			WorktreeDir: filepath.Join(repoRoot, cfg.WorktreeDir),
		})
		s.Stop()
		if err != nil {
			return err
		}

		if len(res.Rows) == 0 {
			msg := "No worktrees found."
			if opts.dirty || opts.behind || opts.typeFilter != "" || opts.service != "" {
				msg = "No worktrees match the given filters."
			}
			return listRenderEmpty(cmd, msg)
		}

		if isJSON(cmd) {
			return listRenderJSON(cmd, res.Rows, res.PRInfos)
		}
		listRenderTable(cmd, res.Rows, opts.full, res.PRInfos, res.GhWarning)
		return nil
	},
}

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

func listArchivedBranches(cmd *cobra.Command, r git.Runner, mainBranch string) error {
	archived, err := operations.ListArchivedBranches(r, mainBranch)
	if err != nil {
		return err
	}

	prefixes := resolver.AllPrefixes()

	if isJSON(cmd) {
		items := make([]output.ListArchivedItem, 0, len(archived))
		for _, b := range archived {
			task, typeName := resolver.TaskAndType(b, prefixes)
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
		task, typeName := resolver.TaskAndType(b, prefixes)

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

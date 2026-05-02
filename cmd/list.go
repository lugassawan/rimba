package cmd

import (
	"os"
	"path/filepath"

	"github.com/lugassawan/rimba/internal/config"
	"github.com/lugassawan/rimba/internal/gh"
	"github.com/lugassawan/rimba/internal/git"
	"github.com/lugassawan/rimba/internal/hint"
	"github.com/lugassawan/rimba/internal/operations"
	"github.com/lugassawan/rimba/internal/spinner"
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
	return validateTypeFilter(typeFilter)
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

	_ = listCmd.RegisterFlagCompletionFunc(flagType, typeFilterCompletion())
}

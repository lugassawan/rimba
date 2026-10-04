package cmd

import (
	"fmt"
	"path/filepath"

	"github.com/lugassawan/rimba/internal/config"
	"github.com/lugassawan/rimba/internal/git"
	"github.com/lugassawan/rimba/internal/hint"
	"github.com/lugassawan/rimba/internal/operations"
	"github.com/lugassawan/rimba/internal/spinner"
	"github.com/spf13/cobra"
)

const (
	flagAs = "as"

	hintAs = "Use a custom name instead of auto-suffix (-1, -2, etc.)"
)

var duplicateCmd = &cobra.Command{
	Use:   "duplicate <task>",
	Short: "Create a new worktree from an existing worktree",
	Long:  "Creates a new worktree branched from an existing worktree's branch, inheriting its prefix. Auto-suffixes with -1, -2, etc. unless --as is provided. Use --dry-run to preview what would be created without making changes.",
	Example: `  rimba duplicate auth             # duplicate auth worktree (auto-suffix)
  rimba duplicate auth --as copy    # duplicate with custom name
  rimba duplicate auth --dry-run    # preview without duplicating`,
	Args: cobra.ExactArgs(1),
	ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if len(args) != 0 {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		return completeWorktreeTasks(cmd, toComplete), cobra.ShellCompDirectiveNoFileComp
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		rawTask := args[0]
		cfg := config.FromContext(cmd.Context())
		ctx := cmd.Context()
		r := newRunner(ctx)

		repoRoot, err := git.MainRepoRoot(ctx, r)
		if err != nil {
			return err
		}
		wt, err := findWorktree(ctx, r, rawTask)
		if err != nil {
			return err
		}

		ps := cfg.PrefixSet()
		_, sourceTask := operations.ResolveTaskInput(rawTask, repoRoot, ps)
		as, _ := cmd.Flags().GetString(flagAs)
		dryRun, _ := cmd.Flags().GetBool(flagDryRun)
		skipDeps, _ := cmd.Flags().GetBool(flagSkipDeps)
		skipHooks, _ := cmd.Flags().GetBool(flagSkipHooks)
		var configModules []config.ModuleConfig
		if cfg.Deps != nil {
			configModules = cfg.Deps.Modules
		}
		postCreateOptions := operations.PostCreateOptions{
			RepoRoot:      repoRoot,
			WorktreeDir:   filepath.Join(repoRoot, cfg.WorktreeDir),
			CopyFiles:     cfg.CopyFiles,
			SkipDeps:      skipDeps,
			AutoDetect:    cfg.IsAutoDetectDeps(),
			ConfigModules: configModules,
			SkipHooks:     skipHooks,
			PostCreate:    cfg.PostCreate,
			Concurrency:   cfg.DepsConcurrency(),
		}
		params := operations.DuplicateParams{
			Source:            wt,
			SourceTask:        sourceTask,
			As:                as,
			PrefixSet:         ps,
			DefaultSource:     cfg.DefaultSource,
			DryRun:            dryRun,
			PostCreateOptions: postCreateOptions,
		}

		hint.New(cmd, hintPainter(cmd)).
			Add(flagSkipDeps, hintSkipDeps).
			Add(flagSkipHooks, hintSkipHooks).
			Add(flagAs, hintAs).
			Add(flagDryRun, hintDryRun).
			Show()

		if dryRun {
			result, err := operations.DuplicateWorktree(ctx, r, params, nil)
			if err != nil {
				return err
			}
			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "[dry-run] would create worktree: %s (branch %s from %s)\n", result.Path, result.Branch, result.SourceBranch)
			if len(cfg.CopyFiles) > 0 {
				fmt.Fprintf(out, "[dry-run] would copy files: %v\n", cfg.CopyFiles)
			}
			if !skipDeps {
				fmt.Fprintf(out, "[dry-run] would install deps\n")
			}
			if !skipHooks && len(cfg.PostCreate) > 0 {
				fmt.Fprintf(out, "[dry-run] would run post-create hooks\n")
			}
			return nil
		}

		if err := ensureTrust(cmd, repoRoot, cfg); err != nil {
			return err
		}
		s := spinner.New(spinnerOpts(cmd))
		defer s.Stop()
		s.Start("Creating worktree...")
		result, err := operations.DuplicateWorktree(ctx, r, params, func(msg string) { s.Update(msg) })
		if err != nil {
			return err
		}
		s.Stop()

		out := cmd.OutOrStdout()
		fmt.Fprintf(out, "Duplicated worktree %q as %q\n", sourceTask, result.Task)
		fmt.Fprintf(out, "  Branch: %s\n", result.Branch)
		fmt.Fprintf(out, "  Path:   %s\n", result.Path)
		if len(result.Copied) > 0 {
			fmt.Fprintf(out, "  Copied: %v\n", result.Copied)
		}
		if len(result.Skipped) > 0 {
			fmt.Fprintf(out, "  Skipped (not found): %v\n", result.Skipped)
		}
		if len(result.SkippedSymlinks) > 0 {
			fmt.Fprintf(out, "  Skipped (symlinks): %v\n", result.SkippedSymlinks)
		}
		printInstallResults(out, result.DepsResults)
		printHookResultsList(out, result.HookResults)
		return nil
	},
}

func init() {
	duplicateCmd.Flags().String(flagAs, "", "custom name for the duplicate worktree")
	duplicateCmd.Flags().Bool(flagSkipDeps, false, "skip dependency detection and installation")
	duplicateCmd.Flags().Bool(flagSkipHooks, false, "skip post-create hooks")
	duplicateCmd.Flags().Bool(flagDryRun, false, "preview what would be duplicated without making changes")
	rootCmd.AddCommand(duplicateCmd)
}

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
	flagSource = "source"

	hintSource = "Branch from a specific source instead of the default branch"
)

func init() {
	addPrefixFlags(addCmd)
	addCmd.Flags().StringP(flagSource, "s", "", "Source branch to create worktree from (default from config)")
	addCmd.Flags().Bool(flagSkipDeps, false, "Skip dependency detection and installation")
	addCmd.Flags().Bool(flagSkipHooks, false, "Skip post-create hooks")
	_ = addCmd.RegisterFlagCompletionFunc(flagSource, func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return completeBranchNames(cmd, toComplete), cobra.ShellCompDirectiveNoFileComp
	})
	rootCmd.AddCommand(addCmd)
}

var addCmd = &cobra.Command{
	Use:   "add <task>",
	Short: "Create a new worktree for a task",
	Long:  "Creates a new git worktree with a branch named <prefix><task> and copies dotfiles from the repo root.",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		task := args[0]
		cfg := config.FromContext(cmd.Context())

		r := newRunner()

		repoRoot, err := git.MainRepoRoot(r)
		if err != nil {
			return err
		}

		prefix := resolvedPrefixString(cmd)

		source, _ := cmd.Flags().GetString(flagSource)
		if source == "" {
			source = cfg.DefaultSource
		}

		skipDeps, _ := cmd.Flags().GetBool(flagSkipDeps)
		skipHooks, _ := cmd.Flags().GetBool(flagSkipHooks)

		hint.New(cmd, hintPainter(cmd)).
			Add(flagSkipDeps, hintSkipDeps).
			Add(flagSkipHooks, hintSkipHooks).
			Add(flagSource, hintSource).
			Show()

		s := spinner.New(spinnerOpts(cmd))
		defer s.Stop()

		var configModules []config.ModuleConfig
		if cfg.Deps != nil {
			configModules = cfg.Deps.Modules
		}

		result, err := operations.AddWorktree(r, operations.AddParams{
			Task:          task,
			Prefix:        prefix,
			Source:        source,
			RepoRoot:      repoRoot,
			WorktreeDir:   filepath.Join(repoRoot, cfg.WorktreeDir),
			CopyFiles:     cfg.CopyFiles,
			SkipDeps:      skipDeps,
			AutoDetect:    cfg.IsAutoDetectDeps(),
			ConfigModules: configModules,
			SkipHooks:     skipHooks,
			PostCreate:    cfg.PostCreate,
		}, func(msg string) { s.Start(msg) })
		if err != nil {
			return err
		}

		out := cmd.OutOrStdout()
		fmt.Fprintf(out, "Created worktree for task %q\n", task)
		fmt.Fprintf(out, "  Branch: %s\n", result.Branch)
		fmt.Fprintf(out, "  Path:   %s\n", result.Path)
		if len(result.Copied) > 0 {
			fmt.Fprintf(out, "  Copied: %v\n", result.Copied)
		}
		if len(result.Skipped) > 0 {
			fmt.Fprintf(out, "  Skipped (not found): %v\n", result.Skipped)
		}

		printInstallResults(out, result.DepsResults)
		printHookResultsList(out, result.HookResults)

		return nil
	},
}

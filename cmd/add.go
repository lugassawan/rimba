package cmd

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strconv"

	"github.com/lugassawan/rimba/internal/config"
	"github.com/lugassawan/rimba/internal/gh"
	"github.com/lugassawan/rimba/internal/git"
	"github.com/lugassawan/rimba/internal/hint"
	"github.com/lugassawan/rimba/internal/operations"
	"github.com/lugassawan/rimba/internal/resolver"
	"github.com/lugassawan/rimba/internal/spinner"
	"github.com/spf13/cobra"
)

const (
	flagSource = "source"
	flagTask   = "task"

	hintSource = "Branch from a specific source instead of the default branch"
)

var prArgRe = regexp.MustCompile(`^pr:(\d+)$`)

var addCmd = &cobra.Command{
	Use:   "add <task|pr:<num>> or add <service>/<task>",
	Short: "Create a new worktree for a task or GitHub PR",
	Long:  "Create a worktree, copy files, install dependencies, and run hooks.\nUse <service>/<task> to scope to a specific service in a monorepo.\nUse pr:<num> to create a worktree from a GitHub PR's head branch.",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg := config.FromContext(cmd.Context())

		r := newRunner()

		repoRoot, err := git.MainRepoRoot(r)
		if err != nil {
			return err
		}

		// pr:<num> shorthand — create worktree from a GitHub PR.
		if m := prArgRe.FindStringSubmatch(args[0]); m != nil {
			prNum, _ := strconv.Atoi(m[1])
			taskOverride, _ := cmd.Flags().GetString(flagTask)

			s := spinner.New(spinnerOpts(cmd))
			defer s.Stop()
			s.Start("Fetching PR metadata...")

			var configModules []config.ModuleConfig
			if cfg.Deps != nil {
				configModules = cfg.Deps.Modules
			}

			skipDeps, _ := cmd.Flags().GetBool(flagSkipDeps)
			skipHooks, _ := cmd.Flags().GetBool(flagSkipHooks)

			result, err := operations.AddPRWorktree(cmd.Context(), r, gh.Default(), operations.AddPRParams{
				PRNumber:      prNum,
				TaskOverride:  taskOverride,
				RepoRoot:      repoRoot,
				WorktreeDir:   filepath.Join(repoRoot, cfg.WorktreeDir),
				CopyFiles:     cfg.CopyFiles,
				SkipDeps:      skipDeps,
				AutoDetect:    cfg.IsAutoDetectDeps(),
				ConfigModules: configModules,
				SkipHooks:     skipHooks,
				PostCreate:    cfg.PostCreate,
				Concurrency:   cfg.DepsConcurrency(),
			}, func(msg string) { s.Update(msg) })
			if err != nil {
				return err
			}

			s.Stop()
			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "Created worktree for PR #%d\n", prNum)
			fmt.Fprintf(out, "  Branch: %s\n", result.Branch)
			fmt.Fprintf(out, "  Path:   %s\n", result.Path)
			if len(result.Copied) > 0 {
				fmt.Fprintf(out, "  Copied: %v\n", result.Copied)
			}
			printInstallResults(out, result.DepsResults)
			printHookResultsList(out, result.HookResults)
			return nil
		}

		service, task := operations.ResolveTaskInput(args[0], repoRoot)
		prefix := resolvedPrefixString(cmd)

		if !hasExplicitPrefixFlag(cmd) {
			if candidate, _ := resolver.SplitServiceInput(args[0]); resolver.ValidPrefixType(candidate) {
				if p, ok := resolver.PrefixString(resolver.PrefixType(candidate)); ok {
					prefix = p
				}
			}
		}

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

		s.Start("Creating worktree...")
		result, err := operations.AddWorktree(r, operations.AddParams{
			Task:          task,
			Service:       service,
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
			Concurrency:   cfg.DepsConcurrency(),
		}, func(msg string) { s.Update(msg) })
		if err != nil {
			return err
		}

		s.Stop()

		out := cmd.OutOrStdout()
		if result.Service != "" {
			fmt.Fprintf(out, "Created worktree for task %q (service: %s)\n", task, result.Service)
		} else {
			fmt.Fprintf(out, "Created worktree for task %q\n", task)
		}
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

func init() {
	addPrefixFlags(addCmd)
	addCmd.Flags().StringP(flagSource, "s", "", "Source branch to create worktree from (default from config)")
	addCmd.Flags().String(flagTask, "", "Override auto-derived task name (pr:<num> mode only)")
	addCmd.Flags().Bool(flagSkipDeps, false, "Skip dependency detection and installation")
	addCmd.Flags().Bool(flagSkipHooks, false, "Skip post-create hooks")
	_ = addCmd.RegisterFlagCompletionFunc(flagSource, func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return completeBranchNames(cmd, toComplete), cobra.ShellCompDirectiveNoFileComp
	})
	rootCmd.AddCommand(addCmd)
}

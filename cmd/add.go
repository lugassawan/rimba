package cmd

import (
	"errors"
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

		skipDeps, _ := cmd.Flags().GetBool(flagSkipDeps)
		skipHooks, _ := cmd.Flags().GetBool(flagSkipHooks)

		var configModules []config.ModuleConfig
		if cfg.Deps != nil {
			configModules = cfg.Deps.Modules
		}

		s := spinner.New(spinnerOpts(cmd))
		defer s.Stop()

		// pr:<num> shorthand — create worktree from a GitHub PR.
		if m := prArgRe.FindStringSubmatch(args[0]); m != nil {
			prNum, _ := strconv.Atoi(m[1])
			taskOverride, _ := cmd.Flags().GetString(flagTask)

			result, err := operations.AddPRWorktree(cmd.Context(), r, gh.Default(), operations.AddPRParams{
				PRNumber:     prNum,
				TaskOverride: taskOverride,
				PostCreateOptions: operations.PostCreateOptions{
					RepoRoot:      repoRoot,
					WorktreeDir:   filepath.Join(repoRoot, cfg.WorktreeDir),
					CopyFiles:     cfg.CopyFiles,
					SkipDeps:      skipDeps,
					AutoDetect:    cfg.IsAutoDetectDeps(),
					ConfigModules: configModules,
					SkipHooks:     skipHooks,
					PostCreate:    cfg.PostCreate,
					Concurrency:   cfg.DepsConcurrency(),
				},
			}, func(msg string) { s.Update(msg) })
			if err != nil {
				return err
			}

			s.Stop()
			printWorktreeResult(cmd, fmt.Sprintf("Created worktree for PR #%d", prNum), result)
			return nil
		}

		if cmd.Flags().Changed(flagTask) {
			return errors.New("--task requires a pr:<num> argument")
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

		hint.New(cmd, hintPainter(cmd)).
			Add(flagSkipDeps, hintSkipDeps).
			Add(flagSkipHooks, hintSkipHooks).
			Add(flagSource, hintSource).
			Show()

		s.Start("Creating worktree...")
		result, err := operations.AddWorktree(r, operations.AddParams{
			Task:    task,
			Service: service,
			Prefix:  prefix,
			Source:  source,
			PostCreateOptions: operations.PostCreateOptions{
				RepoRoot:      repoRoot,
				WorktreeDir:   filepath.Join(repoRoot, cfg.WorktreeDir),
				CopyFiles:     cfg.CopyFiles,
				SkipDeps:      skipDeps,
				AutoDetect:    cfg.IsAutoDetectDeps(),
				ConfigModules: configModules,
				SkipHooks:     skipHooks,
				PostCreate:    cfg.PostCreate,
				Concurrency:   cfg.DepsConcurrency(),
			},
		}, func(msg string) { s.Update(msg) })
		if err != nil {
			return err
		}

		s.Stop()

		header := fmt.Sprintf("Created worktree for task %q", task)
		if result.Service != "" {
			header = fmt.Sprintf("Created worktree for task %q (service: %s)", task, result.Service)
		}
		printWorktreeResult(cmd, header, result)

		return nil
	},
}

func printWorktreeResult(cmd *cobra.Command, header string, result operations.AddResult) {
	out := cmd.OutOrStdout()
	fmt.Fprintln(out, header)
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

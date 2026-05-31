package cmd

import (
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"strconv"

	"github.com/lugassawan/rimba/internal/config"
	"github.com/lugassawan/rimba/internal/errhint"
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
var branchArgRe = regexp.MustCompile(`^branch:(.+)$`)

var newGHRunner = gh.Default

var addCmd = &cobra.Command{
	Use:   "add <task|pr:<num>|branch:<branch>> or add <service>/<task>",
	Short: "Create a new worktree for a task, GitHub PR, or promote the current branch",
	Long: `Create a worktree, copy files, install dependencies, and run hooks.
Use <service>/<task> to scope to a specific service in a monorepo.
Use pr:<num> to create a worktree from a GitHub PR's head branch.
Use branch:<branch> to promote the current branch into its own worktree,
transferring any dirty working-tree state via git stash.

pr:<num> requires gh installed and authenticated. For cross-fork PRs, rimba adds a
gh-fork-<owner> remote automatically. Without --task, the task name is derived as
review/<num>-<slug>. The --task flag is only valid in pr:<num> mode.
branch:<branch> requires that <branch> is the currently checked-out branch in the
main repo and is not the default branch. --source is not valid in branch: mode.`,
	Example: `  rimba add my-feature
  rimba add my-feature --bugfix          # use bugfix/ prefix
  rimba add auth-api/my-feature          # monorepo service scope
  rimba add pr:123                       # create worktree from PR #123
  rimba add pr:123 --task review/auth    # override auto-derived task name
  rimba add branch:feature/my-feature   # promote current branch to worktree`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg := config.FromContext(cmd.Context())

		r := newRunner()

		repoRoot, err := git.MainRepoRoot(r)
		if err != nil {
			return err
		}

		skipDeps, _ := cmd.Flags().GetBool(flagSkipDeps)
		skipHooks, _ := cmd.Flags().GetBool(flagSkipHooks)
		postOpts := buildPostCreateOptions(cfg, repoRoot, skipDeps, skipHooks)

		s := spinner.New(spinnerOpts(cmd))
		defer s.Stop()

		if m := prArgRe.FindStringSubmatch(args[0]); m != nil {
			prNum, _ := strconv.Atoi(m[1])
			return runAddPR(cmd, r, newGHRunner(), prNum, postOpts, s)
		}

		if m := branchArgRe.FindStringSubmatch(args[0]); m != nil {
			if cmd.Flags().Changed(flagSource) {
				return errhint.WithFix(
					errors.New("--source is not valid in branch: mode"),
					"remove the --source flag: branch: promotes an existing branch, not a new one",
				)
			}
			return runAddBranch(cmd, r, cfg, repoRoot, m[1])
		}

		if cmd.Flags().Changed(flagTask) {
			return errhint.WithFix(
				errors.New("--task requires a pr:<num> argument"),
				"pass a PR argument: rimba add pr:<num> --task <name>",
			)
		}

		return runAddTask(cmd, r, args[0], cfg, repoRoot, postOpts, s)
	},
}

func runAddPR(cmd *cobra.Command, r git.Runner, ghR gh.Runner, prNum int, postOpts operations.PostCreateOptions, s *spinner.Spinner) error {
	taskOverride, _ := cmd.Flags().GetString(flagTask)

	result, err := operations.AddPRWorktree(cmd.Context(), r, ghR, operations.AddPRParams{
		PRNumber:          prNum,
		TaskOverride:      taskOverride,
		PostCreateOptions: postOpts,
	}, func(msg string) { s.Update(msg) })
	if err != nil {
		return err
	}

	s.Stop()
	printWorktreeResult(cmd, fmt.Sprintf("Created worktree for PR #%d", prNum), result)
	return nil
}

func runAddTask(cmd *cobra.Command, r git.Runner, arg string, cfg *config.Config, repoRoot string, postOpts operations.PostCreateOptions, s *spinner.Spinner) error {
	service, task := operations.ResolveTaskInput(arg, repoRoot)
	prefix := resolvedPrefixString(cmd)

	if !hasExplicitPrefixFlag(cmd) {
		if candidate, _ := resolver.SplitServiceInput(arg); resolver.ValidPrefixType(candidate) {
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
	result, err := operations.AddWorktree(cmd.Context(), r, operations.AddParams{
		Task:              task,
		Service:           service,
		Prefix:            prefix,
		Source:            source,
		PostCreateOptions: postOpts,
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
}

func runAddBranch(cmd *cobra.Command, r git.Runner, cfg *config.Config, repoRoot, branch string) error {
	s := spinner.New(spinnerOpts(cmd))
	defer s.Stop()

	wtDir := filepath.Join(repoRoot, cfg.WorktreeDir)
	s.Start("Promoting branch to worktree...")
	wtPath, err := operations.PromoteBranch(cmd.Context(), wtDir, r, repoRoot, branch)
	if err != nil {
		return err
	}
	s.Stop()

	out := cmd.OutOrStdout()
	fmt.Fprintf(out, "Promoted branch %q to worktree\n", branch)
	fmt.Fprintf(out, "  Branch: %s\n", branch)
	fmt.Fprintf(out, "  Path:   %s\n", wtPath)
	return nil
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

func buildPostCreateOptions(cfg *config.Config, repoRoot string, skipDeps, skipHooks bool) operations.PostCreateOptions {
	var configModules []config.ModuleConfig
	if cfg.Deps != nil {
		configModules = cfg.Deps.Modules
	}
	return operations.PostCreateOptions{
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
}

func init() {
	addPrefixFlags(addCmd)
	addCmd.Flags().StringP(flagSource, "s", "", "source branch to create worktree from (default from config)")
	addCmd.Flags().String(flagTask, "", "override auto-derived task name (pr:<num> mode only)")
	addCmd.Flags().Bool(flagSkipDeps, false, "skip dependency detection and installation")
	addCmd.Flags().Bool(flagSkipHooks, false, "skip post-create hooks")
	_ = addCmd.RegisterFlagCompletionFunc(flagSource, func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return completeBranchNames(cmd, toComplete), cobra.ShellCompDirectiveNoFileComp
	})
	rootCmd.AddCommand(addCmd)
}

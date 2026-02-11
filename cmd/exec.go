package cmd

import (
	"fmt"
	"strings"

	"github.com/lugassawan/rimba/internal/config"
	"github.com/lugassawan/rimba/internal/executor"
	"github.com/lugassawan/rimba/internal/git"
	"github.com/lugassawan/rimba/internal/hint"
	"github.com/lugassawan/rimba/internal/resolver"
	"github.com/lugassawan/rimba/internal/spinner"
	"github.com/lugassawan/rimba/internal/termcolor"
	"github.com/spf13/cobra"
)

const (
	flagFailFast    = "fail-fast"
	flagConcurrency = "concurrency"

	hintExecAll         = "Target all worktrees"
	hintExecType        = "Filter by prefix type (feature, bugfix, hotfix, etc.)"
	hintExecDirty       = "Only target dirty worktrees"
	hintExecFailFast    = "Stop on first non-zero exit"
	hintExecConcurrency = "Number of parallel workers (default 4)"
)

func init() {
	execCmd.Flags().Bool(flagAll, false, "Target all worktrees")
	execCmd.Flags().String(flagType, "", "Filter by prefix type")
	execCmd.Flags().Bool(flagDirty, false, "Only target dirty worktrees")
	execCmd.Flags().Bool(flagFailFast, false, "Stop on first non-zero exit")
	execCmd.Flags().Int(flagConcurrency, 4, "Number of parallel workers")

	_ = execCmd.RegisterFlagCompletionFunc(flagType, func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		var types []string
		for _, p := range resolver.AllPrefixes() {
			t := strings.TrimSuffix(p, "/")
			if strings.HasPrefix(t, toComplete) {
				types = append(types, t)
			}
		}
		return types, cobra.ShellCompDirectiveNoFileComp
	})

	rootCmd.AddCommand(execCmd)
}

var execCmd = &cobra.Command{
	Use:   "exec <command> [flags]",
	Short: "Run a command across worktrees",
	Long: `Executes a command in multiple worktrees in parallel.

Examples:
  rimba exec "npm test" --all
  rimba exec "go vet ./..." --type feature
  rimba exec "git log --oneline -3" --dirty
  rimba exec "make build" --all --fail-fast --concurrency 8`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg := config.FromContext(cmd.Context())
		r := newRunner()

		all, _ := cmd.Flags().GetBool(flagAll)
		typeFilter, _ := cmd.Flags().GetString(flagType)
		dirtyOnly, _ := cmd.Flags().GetBool(flagDirty)
		failFast, _ := cmd.Flags().GetBool(flagFailFast)
		concurrency, _ := cmd.Flags().GetInt(flagConcurrency)

		if !all && typeFilter == "" && !dirtyOnly {
			return fmt.Errorf("provide at least one filter: --all, --type, or --dirty")
		}

		if typeFilter != "" && !resolver.ValidPrefixType(typeFilter) {
			valid := make([]string, 0, len(resolver.AllPrefixes()))
			for _, p := range resolver.AllPrefixes() {
				valid = append(valid, strings.TrimSuffix(p, "/"))
			}
			return fmt.Errorf("invalid type %q; valid types: %s", typeFilter, strings.Join(valid, ", "))
		}

		hint.New(cmd, hintPainter(cmd)).
			Add(flagAll, hintExecAll).
			Add(flagType, hintExecType).
			Add(flagDirty, hintExecDirty).
			Add(flagFailFast, hintExecFailFast).
			Add(flagConcurrency, hintExecConcurrency).
			Show()

		s := spinner.New(spinnerOpts(cmd))
		defer s.Stop()
		s.Start("Collecting worktrees...")

		targets, err := collectExecTargets(r, cfg, typeFilter, dirtyOnly)
		if err != nil {
			return err
		}

		if len(targets) == 0 {
			s.Stop()
			fmt.Fprintln(cmd.OutOrStdout(), "No worktrees match the given filters.")
			return nil
		}

		s.Update(fmt.Sprintf("Running across %d worktree(s)...", len(targets)))

		results := executor.Run(cmd.Context(), executor.Config{
			Concurrency: concurrency,
			FailFast:    failFast,
			Targets:     targets,
			Command:     []string{"sh", "-c", args[0]},
		})

		s.Stop()
		printExecResults(cmd, results)
		return nil
	},
}

func collectExecTargets(r git.Runner, cfg *config.Config, typeFilter string, dirtyOnly bool) ([]executor.Target, error) {
	entries, err := git.ListWorktrees(r)
	if err != nil {
		return nil, err
	}

	prefixes := resolver.AllPrefixes()
	var targets []executor.Target

	for _, e := range entries {
		if e.Bare || e.Branch == "" || e.Branch == cfg.DefaultSource {
			continue
		}

		task, matchedPrefix := resolver.TaskFromBranch(e.Branch, prefixes)

		if typeFilter != "" && strings.TrimSuffix(matchedPrefix, "/") != typeFilter {
			continue
		}

		if dirtyOnly {
			dirty, err := git.IsDirty(r, e.Path)
			if err != nil || !dirty {
				continue
			}
		}

		targets = append(targets, executor.Target{
			Task:   task,
			Branch: e.Branch,
			Path:   e.Path,
		})
	}

	return targets, nil
}

func printExecResults(cmd *cobra.Command, results []executor.Result) {
	noColor, _ := cmd.Flags().GetBool(flagNoColor)
	p := termcolor.NewPainter(noColor)

	for _, r := range results {
		header := fmt.Sprintf("── %s (%s) ", r.Target.Task, r.Target.Branch)
		switch {
		case r.Err != nil:
			header += p.Paint("error", termcolor.Red)
		case r.ExitCode == 0:
			header += p.Paint("ok", termcolor.Green)
		default:
			header += p.Paint(fmt.Sprintf("exit %d", r.ExitCode), termcolor.Red)
		}
		header += fmt.Sprintf(" [%s]", r.Duration.Round(1e6))

		fmt.Fprintln(cmd.OutOrStdout(), header)

		if out := strings.TrimSpace(r.Stdout); out != "" {
			fmt.Fprintln(cmd.OutOrStdout(), out)
		}
		if out := strings.TrimSpace(r.Stderr); out != "" {
			fmt.Fprintln(cmd.OutOrStdout(), p.Paint(out, termcolor.Yellow))
		}
		fmt.Fprintln(cmd.OutOrStdout())
	}

	// Summary line
	var passed, failed, errored int
	for _, r := range results {
		switch {
		case r.Err != nil:
			errored++
		case r.ExitCode == 0:
			passed++
		default:
			failed++
		}
	}

	summary := fmt.Sprintf("%d passed", passed)
	if failed > 0 {
		summary += fmt.Sprintf(", %d failed", failed)
	}
	if errored > 0 {
		summary += fmt.Sprintf(", %d error(s)", errored)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Exec: %s (%d worktree(s))\n", summary, len(results))
}

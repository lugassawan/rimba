package cmd

import (
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"

	"github.com/lugassawan/rimba/internal/config"
	"github.com/lugassawan/rimba/internal/executor"
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
	flagFailFast    = "fail-fast"
	flagConcurrency = "concurrency"

	hintExecAll     = "Run command in all eligible worktrees"
	hintExecType    = "Filter by prefix type (feature, bugfix, hotfix, etc.)"
	hintExecDirty   = "Run only in worktrees with uncommitted changes"
	hintFailFast    = "Stop execution after the first failure"
	hintConcurrency = "Limit the number of parallel executions"
)

var execCmd = &cobra.Command{
	Use:   "exec <command>",
	Short: "Run a shell command across worktrees",
	Long:  "Executes a shell command in parallel across matching worktrees. Use --all to target all worktrees, or --type to filter by prefix type.",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		opts := execReadFlags(cmd)
		if err := execValidateFlags(opts); err != nil {
			return err
		}

		execShowHints(cmd)

		r := newRunner()
		s := spinner.New(spinnerOpts(cmd))
		defer s.Stop()
		s.Start("Collecting worktrees...")

		prefixes := resolver.AllPrefixes()
		filtered, err := execSelectWorktrees(cmd, r, s, opts, prefixes)
		if err != nil {
			return err
		}

		if len(filtered) == 0 {
			s.Stop()
			fmt.Fprintln(cmd.OutOrStdout(), "No worktrees match the given filters.")
			return nil
		}

		targets := execBuildTargets(filtered, prefixes)
		s.Update(fmt.Sprintf("Running in %d worktree(s)...", len(targets)))

		results := executor.Run(cmd.Context(), executor.Config{
			Targets:     targets,
			Command:     args[0],
			Concurrency: opts.concurrency,
			FailFast:    opts.failFast,
			Runner:      executor.ShellRunner(),
		})

		s.Stop()

		if isJSON(cmd) {
			return execRenderJSON(cmd, args[0], results)
		}
		return execRenderText(cmd, results, prefixes)
	},
}

// execOpts holds parsed exec flags.
type execOpts struct {
	all         bool
	typeFilter  string
	dirty       bool
	failFast    bool
	concurrency int
}

func execReadFlags(cmd *cobra.Command) execOpts {
	all, _ := cmd.Flags().GetBool(flagAll)
	typeFilter, _ := cmd.Flags().GetString(flagType)
	dirty, _ := cmd.Flags().GetBool(flagDirty)
	failFast, _ := cmd.Flags().GetBool(flagFailFast)
	concurrency, _ := cmd.Flags().GetInt(flagConcurrency)
	return execOpts{
		all:         all,
		typeFilter:  typeFilter,
		dirty:       dirty,
		failFast:    failFast,
		concurrency: concurrency,
	}
}

func execValidateFlags(opts execOpts) error {
	if !opts.all && opts.typeFilter == "" {
		return errors.New("provide --all or --type to select worktrees")
	}
	if opts.typeFilter != "" && !resolver.ValidPrefixType(opts.typeFilter) {
		valid := make([]string, 0, len(resolver.AllPrefixes()))
		for _, p := range resolver.AllPrefixes() {
			valid = append(valid, strings.TrimSuffix(p, "/"))
		}
		return fmt.Errorf("invalid type %q; valid types: %s", opts.typeFilter, strings.Join(valid, ", "))
	}
	return nil
}

func execShowHints(cmd *cobra.Command) {
	if isJSON(cmd) {
		return
	}
	hint.New(cmd, hintPainter(cmd)).
		Add(flagAll, hintExecAll).
		Add(flagType, hintExecType).
		Add(flagDirty, hintExecDirty).
		Add(flagFailFast, hintFailFast).
		Add(flagConcurrency, hintConcurrency).
		Show()
}

func execSelectWorktrees(cmd *cobra.Command, r git.Runner, s *spinner.Spinner, opts execOpts, prefixes []string) ([]resolver.WorktreeInfo, error) {
	cfg := config.FromContext(cmd.Context())
	worktrees, err := listWorktreeInfos(r)
	if err != nil {
		return nil, err
	}

	var filtered []resolver.WorktreeInfo
	if opts.typeFilter != "" {
		filtered = operations.FilterByType(worktrees, prefixes, opts.typeFilter)
	} else {
		allTasks := operations.CollectTasks(worktrees, prefixes)
		filtered = operations.FilterEligible(worktrees, prefixes, cfg.DefaultSource, allTasks, true)
	}

	if opts.dirty {
		filtered = filterDirtyWorktrees(r, s, filtered)
	}
	return filtered, nil
}

func execBuildTargets(filtered []resolver.WorktreeInfo, prefixes []string) []executor.Target {
	targets := make([]executor.Target, len(filtered))
	for i, wt := range filtered {
		task, _ := resolver.PureTaskFromBranch(wt.Branch, prefixes)
		targets[i] = executor.Target{
			Path:   wt.Path,
			Branch: wt.Branch,
			Task:   task,
		}
	}
	return targets
}

func execRenderJSON(cmd *cobra.Command, command string, results []executor.Result) error {
	jsonResults := make([]output.ExecResult, len(results))
	for i, r := range results {
		jr := output.ExecResult{
			Task:      r.Target.Task,
			Branch:    r.Target.Branch,
			Path:      r.Target.Path,
			ExitCode:  r.ExitCode,
			Stdout:    string(r.Stdout),
			Stderr:    string(r.Stderr),
			Cancelled: r.Cancelled,
		}
		if r.Err != nil {
			jr.Error = r.Err.Error()
		}
		jsonResults[i] = jr
	}
	data := output.ExecData{
		Command: command,
		Results: jsonResults,
		Success: !hasFailure(results),
	}
	_ = output.WriteJSON(cmd.OutOrStdout(), version, "exec", data)
	if hasFailure(results) {
		return &output.SilentError{ExitCode: 1}
	}
	return nil
}

func execRenderText(cmd *cobra.Command, results []executor.Result, prefixes []string) error {
	noColor, _ := cmd.Flags().GetBool(flagNoColor)
	p := termcolor.NewPainter(noColor)
	printExecResults(cmd, p, results, prefixes)
	if hasFailure(results) {
		return errors.New("one or more commands failed")
	}
	return nil
}

func init() {
	execCmd.Flags().Bool(flagAll, false, "run in all eligible worktrees")
	execCmd.Flags().String(flagType, "", "filter by prefix type (e.g. feature, bugfix)")
	execCmd.Flags().Bool(flagDirty, false, "run only in dirty worktrees")
	execCmd.Flags().Bool(flagFailFast, false, "stop after the first failure")
	execCmd.Flags().Int(flagConcurrency, 0, "max parallel executions (0 = unlimited)")

	_ = execCmd.RegisterFlagCompletionFunc(flagType, func(_ *cobra.Command, _ []string, toComplete string) ([]string, cobra.ShellCompDirective) {
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

// filterDirtyWorktrees filters worktrees to only those with uncommitted changes.
func filterDirtyWorktrees(r git.Runner, s *spinner.Spinner, worktrees []resolver.WorktreeInfo) []resolver.WorktreeInfo {
	isDirty := make([]bool, len(worktrees))
	var wg sync.WaitGroup
	sem := make(chan struct{}, 8)

	for i, wt := range worktrees {
		s.Update(fmt.Sprintf("Checking dirty status... (%d/%d)", i+1, len(worktrees)))
		wg.Add(1)
		go func(idx int, path string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			dirty, err := git.IsDirty(r, path)
			if err == nil && dirty {
				isDirty[idx] = true
			}
		}(i, wt.Path)
	}
	wg.Wait()

	var out []resolver.WorktreeInfo
	for i, wt := range worktrees {
		if isDirty[i] {
			out = append(out, wt)
		}
	}
	return out
}

// printExecResults prints the formatted output for each execution result.
func printExecResults(cmd *cobra.Command, p *termcolor.Painter, results []executor.Result, prefixes []string) {
	out := cmd.OutOrStdout()
	for _, r := range results {
		_, typeName := resolver.TaskAndType(r.Target.Branch, prefixes)

		taskLabel := r.Target.Task
		if c := typeColor(typeName); c != "" {
			taskLabel = p.Paint(taskLabel, c)
		}

		fmt.Fprintf(out, "%s  %s\n", taskLabel, formatExecStatus(r, p))
		printIndentedOutput(out, r)
	}
}

// formatExecStatus returns the colored status string for an execution result.
func formatExecStatus(r executor.Result, p *termcolor.Painter) string {
	switch {
	case r.Cancelled:
		return p.Paint("cancelled", termcolor.Gray)
	case r.Err != nil:
		return p.Paint("error", termcolor.Red)
	case r.ExitCode != 0:
		return p.Paint(fmt.Sprintf("exit %d", r.ExitCode), termcolor.Red)
	default:
		return p.Paint("ok", termcolor.Green)
	}
}

// printIndentedOutput prints stdout/stderr with indentation for a result.
func printIndentedOutput(out io.Writer, r executor.Result) {
	if s := strings.TrimRight(string(r.Stdout), "\n"); s != "" {
		for line := range strings.SplitSeq(s, "\n") {
			fmt.Fprintf(out, "  %s\n", line)
		}
	}

	if r.Err != nil {
		fmt.Fprintf(out, "  %s\n", r.Err)
	} else if s := strings.TrimRight(string(r.Stderr), "\n"); s != "" {
		for line := range strings.SplitSeq(s, "\n") {
			fmt.Fprintf(out, "  %s\n", line)
		}
	}
}

// hasFailure returns true if any result has a non-zero exit code or error.
func hasFailure(results []executor.Result) bool {
	for _, r := range results {
		if r.ExitCode != 0 || r.Err != nil {
			return true
		}
	}
	return false
}

package cmd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync/atomic"

	"github.com/lugassawan/rimba/internal/config"
	"github.com/lugassawan/rimba/internal/errhint"
	"github.com/lugassawan/rimba/internal/executor"
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

const (
	flagFailFast    = "fail-fast"
	flagConcurrency = "concurrency"

	hintExecAll     = "Run command in all eligible worktrees"
	hintExecType    = "Filter by prefix type (feature, bugfix, hotfix, etc.)"
	hintExecDirty   = "Run only in worktrees with uncommitted changes"
	hintFailFast    = "Stop execution after the first failure"
	hintConcurrency = "Limit the number of parallel executions"
)

// execRunner is the injectable executor function type, matching executor.Run.
type execRunner func(context.Context, executor.Config) []executor.Result

var execCmd = &cobra.Command{
	Use:   "exec <command>",
	Short: "Run a shell command across worktrees",
	Long:  "Executes a shell command in parallel across matching worktrees. Use --all to target all worktrees, or --type to filter by prefix type.",
	Example: `  rimba exec --all "git status"
  rimba exec --type bugfix "npm test"`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runExec(cmd, args, newRunner(cmd.Context()), executor.Run)
	},
}

// runExec is the shared implementation for the exec command, accepting
// injected runner and executor so both production and tests use the same path.
func runExec(cmd *cobra.Command, args []string, r git.Runner, execFn execRunner) error {
	opts := execReadFlags(cmd)
	ps := config.PrefixSetFromContext(cmd.Context())
	if err := execValidateFlags(opts, ps); err != nil {
		return err
	}

	execShowHints(cmd)

	s := spinner.New(spinnerOpts(cmd))
	defer s.Stop()
	s.Start("Collecting worktrees...")

	filtered, err := execSelectWorktrees(cmd, r, s, opts, ps)
	if err != nil {
		return err
	}
	filtered = excludeOrphaned(cmd, filtered, ps, defaultSourceFromContext(cmd.Context()))

	if len(filtered) == 0 {
		s.Stop()
		fmt.Fprintln(cmd.OutOrStdout(), "No worktrees match the given filters.")
		return nil
	}

	targets := execBuildTargets(filtered, ps.Strip())
	s.Update(fmt.Sprintf("Running in %d worktree(s)...", len(targets)))

	results := execFn(cmd.Context(), executor.Config{
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
	return execRenderText(cmd, results, ps.Strip())
}

// addExecFlags registers exec-specific flags on c, shared by execCmd and buildExecCmd.
func addExecFlags(c *cobra.Command) {
	c.Flags().Bool(flagAll, false, "run in all eligible worktrees")
	c.Flags().String(flagType, "", "filter by prefix type (e.g. feature, bugfix)")
	c.Flags().Bool(flagDirty, false, "run only in dirty worktrees")
	c.Flags().Bool(flagFailFast, false, "stop after the first failure")
	c.Flags().Int(flagConcurrency, 0, "max parallel executions (0 = unlimited)")
}

// buildExecCmd constructs a testable exec command with injected deps.
// Used by exec_test.go to exercise the full flag-parse → filter → executor pipeline.
func buildExecCmd(r git.Runner, execFn execRunner) *cobra.Command {
	c := &cobra.Command{
		Use:  "exec <command>",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runExec(cmd, args, r, execFn)
		},
	}
	addExecFlags(c)
	c.Flags().Bool(flagJSON, false, "")
	c.Flags().Bool(flagNoColor, false, "")
	return c
}

// execOpts holds parsed exec flags.
type execOpts struct {
	all         bool
	typeFilter  string
	dirty       bool
	failFast    bool
	concurrency int
}

type dirtyResult struct {
	dirty   bool
	warning string
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

func execValidateFlags(opts execOpts, ps *resolver.PrefixSet) error {
	if !opts.all && opts.typeFilter == "" {
		return errhint.WithFix(
			errors.New("provide --all or --type to select worktrees"),
			"run: rimba exec --all <cmd>  OR  rimba exec --type <prefix> <cmd>",
		)
	}
	if err := validateTypeFilter(opts.typeFilter, ps); err != nil {
		return err
	}
	if opts.concurrency < 0 {
		return errhint.WithFix(
			errors.New("--concurrency must be >= 0"),
			"run: rimba exec --concurrency <n>  (n >= 0; 0 = unlimited)",
		)
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

func execSelectWorktrees(cmd *cobra.Command, r git.Runner, s *spinner.Spinner, opts execOpts, ps *resolver.PrefixSet) ([]resolver.WorktreeInfo, error) {
	ctx := cmd.Context()
	cfg := config.FromContext(ctx)
	worktrees, err := listWorktreeInfos(ctx, r)
	if err != nil {
		return nil, err
	}

	var filtered []resolver.WorktreeInfo
	if opts.typeFilter != "" {
		filtered = operations.FilterByType(worktrees, ps, opts.typeFilter)
	} else {
		prefixes := ps.Strip()
		allTasks := operations.CollectTasks(worktrees, prefixes)
		filtered = operations.FilterEligible(worktrees, prefixes, cfg.DefaultSource, allTasks, true)
	}

	if opts.dirty {
		filtered = filterDirtyWorktrees(ctx, cmd, r, s, filtered)
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
	addExecFlags(execCmd)
	_ = execCmd.RegisterFlagCompletionFunc(flagType, typeFilterCompletion())
	rootCmd.AddCommand(execCmd)
}

// excludeOrphaned drops worktrees under a no-longer-configured prefix,
// warning to stderr when any are excluded.
func excludeOrphaned(cmd *cobra.Command, worktrees []resolver.WorktreeInfo, ps *resolver.PrefixSet, mainBranch string) []resolver.WorktreeInfo {
	kept, excluded := operations.FilterOrphaned(worktrees, ps, mainBranch)
	if excluded > 0 {
		fmt.Fprintf(cmd.ErrOrStderr(),
			"Warning: excluding %d worktree(s) with an unrecognized prefix (re-add it to [[resolver.prefix]] to include them)\n",
			excluded)
	}
	return kept
}

// filterDirtyWorktrees filters worktrees to only those with uncommitted changes.
// If IsDirty returns an error for a worktree, it is treated as dirty (included)
// and a warning is emitted to cmd.ErrOrStderr() so the error is visible.
func filterDirtyWorktrees(ctx context.Context, cmd *cobra.Command, r git.Runner, s *spinner.Spinner, worktrees []resolver.WorktreeInfo) []resolver.WorktreeInfo {
	n := len(worktrees)
	var done atomic.Int32

	results := parallel.Collect[dirtyResult](ctx, n, 8, func(ctx context.Context, i int) dirtyResult {
		itemCtx, cancel := git.WithItemTimeout(ctx)
		defer cancel()
		path := worktrees[i].Path
		dirty, err := git.IsDirty(itemCtx, r, path)
		count := done.Add(1)
		s.Update(fmt.Sprintf("Checking dirty status... (%d/%d)", count, n))
		if err != nil {
			return dirtyResult{dirty: true, warning: fmt.Sprintf("Warning: cannot check dirty status for %s: %v", path, err)}
		}
		return dirtyResult{dirty: dirty}
	})

	for _, res := range results {
		if res.warning != "" {
			fmt.Fprintln(cmd.ErrOrStderr(), res.warning)
		}
	}

	var out []resolver.WorktreeInfo
	for i, res := range results {
		if res.dirty {
			out = append(out, worktrees[i])
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

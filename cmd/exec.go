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

type execJSONData struct {
	Command string           `json:"command"`
	Results []execJSONResult `json:"results"`
	Success bool             `json:"success"`
}

type execJSONResult struct {
	Task      string `json:"task"`
	Branch    string `json:"branch"`
	Path      string `json:"path"`
	ExitCode  int    `json:"exit_code"`
	Stdout    string `json:"stdout"`
	Stderr    string `json:"stderr"`
	Error     string `json:"error,omitempty"`
	Cancelled bool   `json:"cancelled,omitempty"`
}

const (
	flagFailFast    = "fail-fast"
	flagConcurrency = "concurrency"

	hintExecAll     = "Run command in all eligible worktrees"
	hintExecType    = "Filter by prefix type (feature, bugfix, hotfix, etc.)"
	hintExecDirty   = "Run only in worktrees with uncommitted changes"
	hintFailFast    = "Stop execution after the first failure"
	hintConcurrency = "Limit the number of parallel executions"
)

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

var execCmd = &cobra.Command{
	Use:   "exec <command>",
	Short: "Run a shell command across worktrees",
	Long:  "Executes a shell command in parallel across matching worktrees. Use --all to target all worktrees, or --type to filter by prefix type.",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg := config.FromContext(cmd.Context())

		r := newRunner()
		all, _ := cmd.Flags().GetBool(flagAll)
		typeFilter, _ := cmd.Flags().GetString(flagType)
		dirty, _ := cmd.Flags().GetBool(flagDirty)
		failFast, _ := cmd.Flags().GetBool(flagFailFast)
		concurrency, _ := cmd.Flags().GetInt(flagConcurrency)

		if !all && typeFilter == "" {
			return errors.New("provide --all or --type to select worktrees")
		}

		if typeFilter != "" && !resolver.ValidPrefixType(typeFilter) {
			valid := make([]string, 0, len(resolver.AllPrefixes()))
			for _, p := range resolver.AllPrefixes() {
				valid = append(valid, strings.TrimSuffix(p, "/"))
			}
			return fmt.Errorf("invalid type %q; valid types: %s", typeFilter, strings.Join(valid, ", "))
		}

		if !isJSON(cmd) {
			hint.New(cmd, hintPainter(cmd)).
				Add(flagAll, hintExecAll).
				Add(flagType, hintExecType).
				Add(flagDirty, hintExecDirty).
				Add(flagFailFast, hintFailFast).
				Add(flagConcurrency, hintConcurrency).
				Show()
		}

		s := spinner.New(spinnerOpts(cmd))
		defer s.Stop()
		s.Start("Collecting worktrees...")

		worktrees, err := listWorktreeInfos(r)
		if err != nil {
			return err
		}

		prefixes := resolver.AllPrefixes()

		var filtered []resolver.WorktreeInfo
		if typeFilter != "" {
			filtered = operations.FilterByType(worktrees, prefixes, typeFilter)
		} else {
			allTasks := operations.CollectTasks(worktrees, prefixes)
			filtered = operations.FilterEligible(worktrees, prefixes, cfg.DefaultSource, allTasks, true)
		}

		if dirty {
			filtered = filterDirtyWorktrees(r, s, filtered)
		}

		if len(filtered) == 0 {
			s.Stop()
			fmt.Fprintln(cmd.OutOrStdout(), "No worktrees match the given filters.")
			return nil
		}

		targets := make([]executor.Target, len(filtered))
		for i, wt := range filtered {
			task, _ := resolver.TaskFromBranch(wt.Branch, prefixes)
			targets[i] = executor.Target{
				Path:   wt.Path,
				Branch: wt.Branch,
				Task:   task,
			}
		}

		s.Update(fmt.Sprintf("Running in %d worktree(s)...", len(targets)))

		results := executor.Run(cmd.Context(), executor.Config{
			Targets:     targets,
			Command:     args[0],
			Concurrency: concurrency,
			FailFast:    failFast,
			Runner:      executor.ShellRunner(),
		})

		s.Stop()

		if isJSON(cmd) {
			jsonResults := make([]execJSONResult, len(results))
			for i, r := range results {
				jr := execJSONResult{
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
			data := execJSONData{
				Command: args[0],
				Results: jsonResults,
				Success: !hasFailure(results),
			}
			_ = output.WriteJSON(cmd.OutOrStdout(), version, "exec", data)
			if hasFailure(results) {
				return &output.SilentError{ExitCode: 1}
			}
			return nil
		}

		noColor, _ := cmd.Flags().GetBool(flagNoColor)
		p := termcolor.NewPainter(noColor)

		printExecResults(cmd, p, results, prefixes)

		if hasFailure(results) {
			return errors.New("one or more commands failed")
		}
		return nil
	},
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
		_, matchedPrefix := resolver.TaskFromBranch(r.Target.Branch, prefixes)
		typeName := strings.TrimSuffix(matchedPrefix, "/")

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

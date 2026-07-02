package cmd

import (
	"context"
	"io"

	"github.com/lugassawan/rimba/internal/config"
	"github.com/lugassawan/rimba/internal/debug"
	"github.com/lugassawan/rimba/internal/gh"
	"github.com/lugassawan/rimba/internal/git"
	"github.com/lugassawan/rimba/internal/operations"
	"github.com/lugassawan/rimba/internal/output"
	"github.com/lugassawan/rimba/internal/resolver"
	"github.com/lugassawan/rimba/internal/spinner"
	"github.com/lugassawan/rimba/internal/termcolor"
	"github.com/spf13/cobra"
)

// newRunner creates a git.Runner for command execution.
// Defined as a variable to allow test overrides (same pattern as newUpdater).
// When RIMBA_DEBUG is set, wraps the runner with timing instrumentation.
// The timeout is sourced from config in ctx; falls back to DefaultCommandTimeout.
var newRunner = func(ctx context.Context) git.Runner {
	timeout := config.DefaultCommandTimeout
	if cfg := config.FromContext(ctx); cfg != nil {
		timeout = cfg.EffectiveCommandTimeout()
	}
	return debug.WrapRunner(&git.ExecRunner{Timeout: timeout})
}

// newGHRunner creates a gh.Runner with a timeout sourced from config in ctx.
var newGHRunner = func(ctx context.Context) gh.Runner {
	timeout := config.DefaultCommandTimeout
	if cfg := config.FromContext(ctx); cfg != nil {
		timeout = cfg.EffectiveCommandTimeout()
	}
	return gh.Default(timeout)
}

// hintPainter returns a termcolor.Painter derived from the cobra command flags.
func hintPainter(cmd *cobra.Command) *termcolor.Painter {
	noColor, _ := cmd.Flags().GetBool(flagNoColor)
	return termcolor.NewPainter(noColor)
}

// isJSON returns true if the --json flag is set on the given command.
func isJSON(cmd *cobra.Command) bool {
	return output.IsJSON(cmd)
}

// spinnerOpts returns spinner options derived from the cobra command flags.
// In JSON mode the spinner is silenced by writing to io.Discard.
func spinnerOpts(cmd *cobra.Command) spinner.Options {
	noColor, _ := cmd.Flags().GetBool(flagNoColor)
	w := cmd.ErrOrStderr()
	if isJSON(cmd) {
		w = io.Discard
	}
	return spinner.Options{Writer: w, NoColor: noColor}
}

// resolveMainBranch tries to get the main branch from config, falling back to DefaultBranch.
func resolveMainBranch(ctx context.Context, r git.Runner) (string, error) {
	repoRoot, err := git.MainRepoRoot(ctx, r)
	if err != nil {
		return "", err
	}

	var configDefault string
	if cfg, err := config.Resolve(repoRoot); err == nil {
		configDefault = cfg.DefaultSource
	}

	return operations.ResolveMainBranch(ctx, r, configDefault)
}

// listWorktreeInfos converts git worktree entries to resolver-compatible WorktreeInfo slice.
func listWorktreeInfos(ctx context.Context, r git.Runner) ([]resolver.WorktreeInfo, error) {
	return operations.ListWorktreeInfos(ctx, r)
}

// findWorktree looks up a worktree by user input (task or service/task).
// It resolves the input to detect monorepo service names.
func findWorktree(ctx context.Context, r git.Runner, input string) (resolver.WorktreeInfo, error) {
	repoRoot, err := git.MainRepoRoot(ctx, r)
	if err != nil {
		return operations.FindWorktree(ctx, r, "", input)
	}
	service, task := operations.ResolveTaskInput(input, repoRoot)
	return operations.FindWorktree(ctx, r, service, task)
}

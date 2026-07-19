package cmd

import (
	"context"
	"fmt"
	"io"
	"path/filepath"

	"github.com/lugassawan/rimba/internal/config"
	"github.com/lugassawan/rimba/internal/gh"
	"github.com/lugassawan/rimba/internal/git"
	"github.com/lugassawan/rimba/internal/observability"
	"github.com/lugassawan/rimba/internal/operations"
	"github.com/lugassawan/rimba/internal/output"
	"github.com/lugassawan/rimba/internal/resolver"
	"github.com/lugassawan/rimba/internal/spinner"
	"github.com/lugassawan/rimba/internal/termcolor"
	"github.com/spf13/cobra"
)

// newRunner creates a git.Runner for command execution.
// Defined as a variable to allow test overrides (same pattern as newUpdater).
// Always wrapped with observability.WrapRunner (see its doc for the
// per-call ctx-derivation rationale). The timeout is sourced from config in
// ctx; falls back to DefaultCommandTimeout.
var newRunner = func(ctx context.Context) git.Runner {
	timeout := config.DefaultCommandTimeout
	if cfg := config.FromContext(ctx); cfg != nil {
		timeout = cfg.EffectiveCommandTimeout()
	}
	return observability.WrapRunner(&git.ExecRunner{Timeout: timeout})
}

// newGHRunner creates a gh.Runner with a timeout sourced from config in ctx.
// Always wrapped with gh.WrapRunner, same per-call design as newRunner.
var newGHRunner = func(ctx context.Context) gh.Runner {
	timeout := config.DefaultCommandTimeout
	if cfg := config.FromContext(ctx); cfg != nil {
		timeout = cfg.EffectiveCommandTimeout()
	}
	return gh.WrapRunner(gh.Default(timeout))
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

// errStr stringifies a non-fatal sub-error for JSON output, returning "" for nil.
func errStr(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

// nonNilStrings guards against nil string slices so they serialize as JSON
// "[]" rather than "null".
func nonNilStrings(s []string) []string {
	if s == nil {
		return make([]string, 0)
	}
	return s
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

// resolveMainBranch returns the repo's default branch, always derived from git.
func resolveMainBranch(ctx context.Context, r git.Runner) (string, error) {
	if _, err := git.MainRepoRoot(ctx, r); err != nil {
		return "", err
	}
	return git.DefaultBranch(ctx, r)
}

// reapConfidentLocks best-effort recovers stale index.lock files left by a
// sweep whose owner is proven dead; a CommonDir failure skips it silently.
func reapConfidentLocks(ctx context.Context, cmd *cobra.Command, r git.Runner) {
	commonDir, err := git.CommonDir(ctx, r)
	if err != nil {
		return
	}
	removals := operations.ReapConfidentLocks(commonDir)
	if len(removals) == 0 || isJSON(cmd) {
		return
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Recovered %d stale index.lock file(s) left by an interrupted sweep.\n", len(removals))
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
	service, task := operations.ResolveTaskInput(input, repoRoot, config.PrefixSetFromContext(ctx))
	return operations.FindWorktree(ctx, r, service, task)
}

// withBestEffortConfig lets skipConfig commands (status/clean/log) pick up
// custom prefixes when reachable, without failing if no repo/config exists.
func withBestEffortConfig(cmd *cobra.Command) context.Context {
	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}
	if config.FromContext(ctx) != nil {
		return ctx // already loaded (e.g. tests injecting config directly)
	}
	r := newRunner(ctx)
	repoRoot, err := git.MainRepoRoot(ctx, r)
	if err != nil {
		return ctx
	}
	cfg, err := config.Resolve(repoRoot)
	if err != nil {
		return ctx
	}
	defaultBranch, err := git.DefaultBranch(ctx, r)
	if err != nil {
		return ctx
	}
	cfg.FillDefaults(filepath.Base(repoRoot), defaultBranch)
	if err := cfg.Validate(); err != nil {
		return ctx
	}
	return config.WithConfig(ctx, cfg)
}

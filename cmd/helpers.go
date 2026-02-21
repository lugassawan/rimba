package cmd

import (
	"io"

	"github.com/lugassawan/rimba/internal/config"
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
var newRunner = func() git.Runner {
	return &git.ExecRunner{}
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
func resolveMainBranch(r git.Runner) (string, error) {
	repoRoot, err := git.MainRepoRoot(r)
	if err != nil {
		return "", err
	}

	var configDefault string
	if cfg, err := config.Resolve(repoRoot); err == nil {
		configDefault = cfg.DefaultSource
	}

	return operations.ResolveMainBranch(r, configDefault)
}

// listWorktreeInfos converts git worktree entries to resolver-compatible WorktreeInfo slice.
func listWorktreeInfos(r git.Runner) ([]resolver.WorktreeInfo, error) {
	return operations.ListWorktreeInfos(r)
}

// findWorktree looks up a worktree by task name.
func findWorktree(r git.Runner, task string) (resolver.WorktreeInfo, error) {
	return operations.FindWorktree(r, task)
}

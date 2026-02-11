package cmd

import (
	"fmt"
	"path/filepath"

	"github.com/lugassawan/rimba/internal/config"
	"github.com/lugassawan/rimba/internal/git"
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

// spinnerOpts returns spinner options derived from the cobra command flags.
func spinnerOpts(cmd *cobra.Command) spinner.Options {
	noColor, _ := cmd.Flags().GetBool(flagNoColor)
	return spinner.Options{Writer: cmd.ErrOrStderr(), NoColor: noColor}
}

// resolveMainBranch tries to get the main branch from config, falling back to DefaultBranch.
func resolveMainBranch(r git.Runner) (string, error) {
	repoRoot, err := git.RepoRoot(r)
	if err != nil {
		return "", err
	}

	cfg, err := config.Load(filepath.Join(repoRoot, config.FileName))
	if err == nil && cfg.DefaultSource != "" {
		return cfg.DefaultSource, nil
	}

	// No config — use git detection
	return git.DefaultBranch(r)
}

// listWorktreeInfos converts git worktree entries to resolver-compatible WorktreeInfo slice.
func listWorktreeInfos(r git.Runner) ([]resolver.WorktreeInfo, error) {
	entries, err := git.ListWorktrees(r)
	if err != nil {
		return nil, err
	}

	worktrees := make([]resolver.WorktreeInfo, len(entries))
	for i, e := range entries {
		worktrees[i] = resolver.WorktreeInfo{
			Path:   e.Path,
			Branch: e.Branch,
		}
	}
	return worktrees, nil
}

// findWorktree looks up a worktree by task name.
func findWorktree(r git.Runner, task string) (resolver.WorktreeInfo, error) {
	worktrees, err := listWorktreeInfos(r)
	if err != nil {
		return resolver.WorktreeInfo{}, err
	}

	wt, found := resolver.FindBranchForTask(task, worktrees, resolver.AllPrefixes())
	if !found {
		return resolver.WorktreeInfo{}, fmt.Errorf(errWorktreeNotFound, task)
	}
	return wt, nil
}

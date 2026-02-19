package cmd

import (
	"sort"
	"strings"

	"github.com/lugassawan/rimba/internal/config"
	"github.com/lugassawan/rimba/internal/git"
	"github.com/lugassawan/rimba/internal/operations"
	"github.com/lugassawan/rimba/internal/resolver"
	"github.com/spf13/cobra"
)

// completeWorktreeTasks returns task names for shell completion.
func completeWorktreeTasks(_ *cobra.Command, toComplete string) []string {
	r := newRunner()
	entries, err := git.ListWorktrees(r)
	if err != nil {
		return nil
	}

	prefixes := resolver.AllPrefixes()
	var tasks []string
	for _, e := range entries {
		if e.Bare || e.Branch == "" {
			continue
		}
		task, _ := resolver.TaskFromBranch(e.Branch, prefixes)
		if strings.HasPrefix(task, toComplete) {
			tasks = append(tasks, task)
		}
	}
	return tasks
}

// completeOpenShortcuts returns shortcut names from [open] config for shell completion.
func completeOpenShortcuts(cmd *cobra.Command, toComplete string) []string {
	cfg := config.FromContext(cmd.Context())
	if cfg == nil || cfg.Open == nil {
		return nil
	}

	var names []string
	for k := range cfg.Open {
		if strings.HasPrefix(k, toComplete) {
			names = append(names, k)
		}
	}
	sort.Strings(names)
	return names
}

// completeArchivedTasks returns task names from archived branches (branches not in any active worktree).
func completeArchivedTasks(_ *cobra.Command, toComplete string) []string {
	r := newRunner()

	mainBranch, _ := resolveMainBranch(r)

	archived, err := operations.ListArchivedBranches(r, mainBranch)
	if err != nil {
		return nil
	}

	prefixes := resolver.AllPrefixes()
	var tasks []string
	for _, b := range archived {
		task, _ := resolver.TaskFromBranch(b, prefixes)
		if strings.HasPrefix(task, toComplete) {
			tasks = append(tasks, task)
		}
	}
	return tasks
}

// completeBranchNames returns branch names for shell completion.
func completeBranchNames(_ *cobra.Command, toComplete string) []string {
	r := newRunner()
	out, err := r.Run("branch", "--format=%(refname:short)")
	if err != nil {
		return nil
	}

	var branches []string
	for b := range strings.SplitSeq(out, "\n") {
		b = strings.TrimSpace(b)
		if b != "" && strings.HasPrefix(b, toComplete) {
			branches = append(branches, b)
		}
	}
	return branches
}

package cmd

import (
	"strings"

	"github.com/lugassawan/rimba/internal/git"
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

package resolver

import (
	"cmp"
	"slices"
	"strings"
)

// WorktreeDetail holds the resolved view of a worktree with extracted task and type.
type WorktreeDetail struct {
	Task   string
	Type   string
	Branch string
	Path   string
	Status string
}

// NewWorktreeDetail constructs a WorktreeDetail by resolving the task name and type
// from the branch using the given prefixes. Path and status are passed through as-is.
func NewWorktreeDetail(branch string, prefixes []string, path, status string) WorktreeDetail {
	task, matchedPrefix := TaskFromBranch(branch, prefixes)

	typeLabel := ""
	if matchedPrefix != "" {
		typeLabel = strings.TrimSuffix(matchedPrefix, "/")
	}

	return WorktreeDetail{
		Task:   task,
		Type:   typeLabel,
		Branch: branch,
		Path:   path,
		Status: status,
	}
}

// SortDetailsByTask sorts a slice of WorktreeDetail by task name ascending.
func SortDetailsByTask(details []WorktreeDetail) {
	slices.SortFunc(details, func(a, b WorktreeDetail) int {
		return cmp.Compare(a.Task, b.Task)
	})
}

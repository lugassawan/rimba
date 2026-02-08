package resolver

import (
	"cmp"
	"fmt"
	"slices"
	"strings"
)

// WorktreeStatus holds the structured git status of a worktree.
type WorktreeStatus struct {
	Dirty  bool
	Ahead  int
	Behind int
}

// WorktreeDetail holds the resolved view of a worktree with extracted task and type.
type WorktreeDetail struct {
	Task      string
	Type      string
	Branch    string
	Path      string
	IsCurrent bool
	Status    WorktreeStatus
}

// NewWorktreeDetail constructs a WorktreeDetail by resolving the task name and type
// from the branch using the given prefixes.
func NewWorktreeDetail(branch string, prefixes []string, path string, status WorktreeStatus, isCurrent bool) WorktreeDetail {
	task, matchedPrefix := TaskFromBranch(branch, prefixes)

	typeLabel := ""
	if matchedPrefix != "" {
		typeLabel = strings.TrimSuffix(matchedPrefix, "/")
	}

	return WorktreeDetail{
		Task:      task,
		Type:      typeLabel,
		Branch:    branch,
		Path:      path,
		IsCurrent: isCurrent,
		Status:    status,
	}
}

// FormatStatus returns a human-readable status string.
// - "[dirty]" when dirty
// - "↑N" when ahead, "↓M" when behind
// - "✓" when fully clean
// Combines as needed, e.g. "[dirty] ↑2 ↓1"
func FormatStatus(s WorktreeStatus) string {
	var parts []string

	if s.Dirty {
		parts = append(parts, "[dirty]")
	}
	if s.Ahead > 0 {
		parts = append(parts, fmt.Sprintf("↑%d", s.Ahead))
	}
	if s.Behind > 0 {
		parts = append(parts, fmt.Sprintf("↓%d", s.Behind))
	}

	if len(parts) == 0 {
		return "✓"
	}
	return strings.Join(parts, " ")
}

// SortDetailsByTask sorts a slice of WorktreeDetail by task name ascending.
func SortDetailsByTask(details []WorktreeDetail) {
	slices.SortFunc(details, func(a, b WorktreeDetail) int {
		return cmp.Compare(a.Task, b.Task)
	})
}

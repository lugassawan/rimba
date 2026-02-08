package resolver

import (
	"slices"
	"strings"
)

// IsInherited checks if a task was created as a duplicate of another task.
// A task matching "<existing-task>-N" where N is a numeric suffix and
// <existing-task> exists in allTasks is considered inherited.
func IsInherited(task string, allTasks []string) bool {
	// Find the last '-' that could precede a numeric suffix
	idx := strings.LastIndex(task, "-")
	if idx <= 0 || idx == len(task)-1 {
		return false
	}

	suffix := task[idx+1:]
	if !isNumeric(suffix) {
		return false
	}

	base := task[:idx]
	return slices.Contains(allTasks, base)
}

// isNumeric reports whether s consists entirely of ASCII digits.
func isNumeric(s string) bool {
	if s == "" {
		return false
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

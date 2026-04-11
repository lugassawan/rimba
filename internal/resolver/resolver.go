package resolver

import (
	"path/filepath"
	"strings"
)

// BranchName returns the full branch name for a task with the given prefix.
func BranchName(prefix, task string) string {
	return prefix + task
}

// FullBranchName constructs the branch name with optional service prefix.
// Monorepo: "auth-api" + "feature/" + "my-task" → "auth-api/feature/my-task"
// Standard: "" + "feature/" + "my-task" → "feature/my-task"
func FullBranchName(service, prefix, task string) string {
	if service != "" {
		return service + "/" + prefix + task
	}
	return prefix + task
}

// SplitServiceInput splits input on the first "/" and returns the candidate
// service name and the rest. Returns ("", input) if no "/" is found.
// The caller is responsible for validating whether candidate is a real service.
func SplitServiceInput(input string) (candidate, rest string) {
	if i := strings.Index(input, "/"); i > 0 {
		return input[:i], input[i+1:]
	}
	return "", input
}

// SanitizeTask replaces "/" with "-" in a task name.
func SanitizeTask(task string) string {
	return strings.ReplaceAll(task, "/", "-")
}

// ServiceFromBranch extracts the service, task, and matched prefix from a branch.
// "auth-api/feature/auth-redirect" → ("auth-api", "auth-redirect", "feature/")
// "feature/auth-redirect"          → ("", "auth-redirect", "feature/")
func ServiceFromBranch(branch string, prefixes []string) (service, task, matchedPrefix string) {
	// Try direct prefix match first (standard mode)
	for _, p := range prefixes {
		if t, ok := strings.CutPrefix(branch, p); ok {
			return "", t, p
		}
	}
	// Try service/prefix/task pattern (monorepo mode)
	if i := strings.Index(branch, "/"); i > 0 {
		rest := branch[i+1:]
		for _, p := range prefixes {
			if t, ok := strings.CutPrefix(rest, p); ok {
				return branch[:i], t, p
			}
		}
	}
	return "", branch, ""
}

// DirName converts a branch name to a directory-safe name.
// e.g. "feature/my-task" → "feature-my-task"
func DirName(branch string) string {
	return strings.ReplaceAll(branch, "/", "-")
}

// WorktreePath returns the full path to the worktree directory.
func WorktreePath(worktreeDir, branch string) string {
	return filepath.Join(worktreeDir, DirName(branch))
}

// TaskFromBranch extracts the task name from a branch by trying each prefix in order.
// Returns the task name and the matched prefix string.
// If no prefix matches, returns the full branch name and an empty string.
func TaskFromBranch(branch string, prefixes []string) (task, matchedPrefix string) {
	for _, p := range prefixes {
		if t, ok := strings.CutPrefix(branch, p); ok {
			return t, p
		}
	}
	return branch, ""
}

// WorktreeInfo holds parsed information about a worktree.
type WorktreeInfo struct {
	Path    string
	Branch  string
	Service string // extracted from branch, empty for standard
}

// FindBranchForTask searches worktrees for one matching the given service and task.
// When service is non-empty, it searches for service/prefix/task patterns.
// When service is empty, it searches for prefix/task patterns and falls back
// to a cross-service task search (returning the match only if unambiguous).
func FindBranchForTask(service, task string, worktrees []WorktreeInfo, prefixes []string) (WorktreeInfo, bool) {
	// Try prefix-based match (with or without service)
	if wt, ok := findByPrefix(service, task, worktrees, prefixes); ok {
		return wt, true
	}

	// Fallback: exact branch name match
	if wt, ok := findExact(task, worktrees); ok {
		return wt, true
	}

	// Fallback: task-only search across all services (monorepo convenience)
	if service == "" {
		matches := FindAllBranchesForTask(task, worktrees, prefixes)
		if len(matches) == 1 {
			return matches[0], true
		}
	}

	return WorktreeInfo{}, false
}

// FindAllBranchesForTask returns all worktrees matching a task name
// across all services. Used for disambiguation.
func FindAllBranchesForTask(task string, worktrees []WorktreeInfo, prefixes []string) []WorktreeInfo {
	var matches []WorktreeInfo
	for _, wt := range worktrees {
		_, t, _ := ServiceFromBranch(wt.Branch, prefixes)
		if t == task {
			matches = append(matches, wt)
		}
	}
	return matches
}

// findByPrefix searches worktrees for a prefix+task match, optionally scoped to a service.
func findByPrefix(service, task string, worktrees []WorktreeInfo, prefixes []string) (WorktreeInfo, bool) {
	for _, p := range prefixes {
		var target string
		if service != "" {
			target = FullBranchName(service, p, task)
		} else {
			target = BranchName(p, task)
		}
		for _, wt := range worktrees {
			if wt.Branch == target {
				return wt, true
			}
		}
	}
	return WorktreeInfo{}, false
}

// findExact searches worktrees for an exact branch name match.
func findExact(task string, worktrees []WorktreeInfo) (WorktreeInfo, bool) {
	for _, wt := range worktrees {
		if wt.Branch == task {
			return wt, true
		}
	}
	return WorktreeInfo{}, false
}

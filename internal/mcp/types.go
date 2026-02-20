package mcp

import (
	"github.com/lugassawan/rimba/internal/resolver"
)

// listItem mirrors the JSON shape from cmd/list.go.
type listItem struct {
	Task      string                  `json:"task"`
	Type      string                  `json:"type"`
	Branch    string                  `json:"branch"`
	Path      string                  `json:"path"`
	IsCurrent bool                    `json:"is_current"`
	Status    resolver.WorktreeStatus `json:"status"`
}

// listArchivedItem mirrors the JSON shape for archived branches.
type listArchivedItem struct {
	Task   string `json:"task"`
	Type   string `json:"type"`
	Branch string `json:"branch"`
}

// statusData mirrors the JSON shape from cmd/status.go.
type statusData struct {
	Summary   statusSummary `json:"summary"`
	Worktrees []statusItem  `json:"worktrees"`
	StaleDays int           `json:"stale_days"`
}

// statusSummary holds aggregate counts.
type statusSummary struct {
	Total  int `json:"total"`
	Dirty  int `json:"dirty"`
	Stale  int `json:"stale"`
	Behind int `json:"behind"`
}

// statusItem holds per-worktree status.
type statusItem struct {
	Task   string                  `json:"task"`
	Type   string                  `json:"type"`
	Branch string                  `json:"branch"`
	Status resolver.WorktreeStatus `json:"status"`
	Age    *statusAge              `json:"age"`
}

// statusAge holds last-commit age information.
type statusAge struct {
	LastCommit string `json:"last_commit"`
	Stale      bool   `json:"stale"`
}

// conflictCheckData mirrors the JSON shape from cmd/conflict_check.go.
type conflictCheckData struct {
	Overlaps      []overlapItem  `json:"overlaps"`
	DryMerges     []dryMergeItem `json:"dry_merges,omitempty"`
	TotalFiles    int            `json:"total_files"`
	TotalBranches int            `json:"total_branches"`
}

// overlapItem represents a file touched by multiple branches.
type overlapItem struct {
	File     string   `json:"file"`
	Branches []string `json:"branches"`
	Severity string   `json:"severity"`
}

// dryMergeItem represents the result of a simulated merge.
type dryMergeItem struct {
	Branch1       string   `json:"branch1"`
	Branch2       string   `json:"branch2"`
	HasConflicts  bool     `json:"has_conflicts"`
	ConflictFiles []string `json:"conflict_files,omitempty"`
}

// execData mirrors the JSON shape from cmd/exec.go.
type execData struct {
	Command string       `json:"command"`
	Results []execResult `json:"results"`
	Success bool         `json:"success"`
}

// execResult holds per-worktree execution results.
type execResult struct {
	Task      string `json:"task"`
	Branch    string `json:"branch"`
	Path      string `json:"path"`
	ExitCode  int    `json:"exit_code"`
	Stdout    string `json:"stdout"`
	Stderr    string `json:"stderr"`
	Error     string `json:"error,omitempty"`
	Cancelled bool   `json:"cancelled,omitempty"`
}

// addResult holds the outcome of a worktree add.
type addResult struct {
	Task   string `json:"task"`
	Branch string `json:"branch"`
	Path   string `json:"path"`
	Source string `json:"source"`
}

// removeResult holds the outcome of a worktree removal.
type removeResult struct {
	Task            string `json:"task"`
	Branch          string `json:"branch"`
	BranchDeleted   bool   `json:"branch_deleted"`
	WorktreeRemoved bool   `json:"worktree_removed"`
}

// mergeResult holds the outcome of a merge operation.
type mergeResult struct {
	Source        string `json:"source"`
	Into          string `json:"into"`
	SourceRemoved bool   `json:"source_removed"`
}

// syncResult holds the outcome of a sync operation.
type syncResult struct {
	Results []syncWorktreeResult `json:"results"`
}

// syncWorktreeResult mirrors operations.SyncWorktreeResult as JSON.
type syncWorktreeResult struct {
	Branch      string `json:"branch"`
	Synced      bool   `json:"synced"`
	Skipped     bool   `json:"skipped"`
	SkipReason  string `json:"skip_reason,omitempty"`
	Failed      bool   `json:"failed"`
	FailureHint string `json:"failure_hint,omitempty"`
	Pushed      bool   `json:"pushed"`
	PushSkipped bool   `json:"push_skipped"`
	PushFailed  bool   `json:"push_failed"`
	PushError   string `json:"push_error,omitempty"`
}

// cleanResult holds the outcome of a clean operation.
type cleanResult struct {
	Mode    string        `json:"mode"`
	DryRun  bool          `json:"dry_run"`
	Removed []cleanedItem `json:"removed"`
	Output  string        `json:"output,omitempty"`
}

// cleanedItem represents a single cleaned branch/worktree.
type cleanedItem struct {
	Branch string `json:"branch"`
	Path   string `json:"path,omitempty"`
}

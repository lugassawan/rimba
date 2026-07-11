package mcp

import (
	"github.com/lugassawan/rimba/internal/output"
)

// Type aliases for shared JSON types from the output package.
// These allow MCP handlers to use the same types as CLI commands.
type (
	listItem         = output.ListItem
	listArchivedItem = output.ListArchivedItem
	statusData       = output.StatusData
	statusSummary    = output.StatusSummary
	statusItem       = output.StatusItem
	statusAge        = output.StatusAge
	execData         = output.ExecData
	execResult       = output.ExecResult
	logItem          = output.LogItem
)

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

// addResult holds the outcome of a worktree add.
type addResult struct {
	Task   string `json:"task,omitempty"`
	Branch string `json:"branch"`
	Path   string `json:"path"`
	Source string `json:"source,omitempty"`
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
	RemoteDeleted bool   `json:"remote_deleted,omitempty"`
}

// syncResult holds the outcome of a sync operation.
type syncResult struct {
	FetchWarning string               `json:"fetch_warning,omitempty"`
	Results      []syncWorktreeResult `json:"results"`
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
	Mode         string        `json:"mode"`
	DryRun       bool          `json:"dry_run"`
	Removed      []cleanedItem `json:"removed"`
	Output       string        `json:"output,omitempty"`
	Warnings     []string      `json:"warnings,omitempty"`
	RemotePruned []string      `json:"remote_pruned,omitempty"`
}

// cleanedItem represents a single cleaned branch/worktree.
type cleanedItem struct {
	Branch        string `json:"branch"`
	Path          string `json:"path,omitempty"`
	RemoteDeleted bool   `json:"remote_deleted,omitempty"`
}

// renameResult holds the outcome of a worktree rename operation.
type renameResult struct {
	OldBranch string `json:"old_branch"`
	NewBranch string `json:"new_branch"`
	OldPath   string `json:"old_path"`
	NewPath   string `json:"new_path"`
	// Push status (only meaningful when push=true).
	Published      bool   `json:"published,omitempty"`
	PublishError   string `json:"publish_error,omitempty"`
	RemoteDeleted  bool   `json:"remote_deleted,omitempty"`
	RemoteError    string `json:"remote_error,omitempty"`
	NoOriginRemote bool   `json:"no_origin_remote,omitempty"`
}

// mergePlanResult holds the recommended merge order.
type mergePlanResult struct {
	Steps []mergePlanStep `json:"steps"`
}

// mergePlanStep represents one step in the recommended merge order.
type mergePlanStep struct {
	Order     int    `json:"order"`
	Task      string `json:"task"`
	Branch    string `json:"branch"`
	Conflicts int    `json:"conflicts"`
}

// logResult holds the last-commit entries for each worktree.
type logResult struct {
	Entries []logItem `json:"entries"`
}

// archiveResult holds the outcome of an archive operation.
type archiveResult struct {
	Path   string   `json:"path"`
	Branch string   `json:"branch"`
	DryRun bool     `json:"dry_run"`
	Steps  []string `json:"steps,omitempty"`
}

// restoreResult holds the outcome of a worktree restore operation.
type restoreResult struct {
	Task            string   `json:"task"`
	Branch          string   `json:"branch"`
	Path            string   `json:"path"`
	Copied          []string `json:"copied,omitempty"`
	Skipped         []string `json:"skipped,omitempty"`
	SkippedSymlinks []string `json:"skipped_symlinks,omitempty"`
}

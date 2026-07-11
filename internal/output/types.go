package output

import "github.com/lugassawan/rimba/internal/resolver"

// ListItem represents a worktree entry in JSON output.
// PRNumber and CIStatus are set under --full; nil means unknown.
type ListItem struct {
	Task      string                  `json:"task"`
	Service   string                  `json:"service,omitempty"`
	Type      string                  `json:"type"`
	Branch    string                  `json:"branch"`
	Path      string                  `json:"path"`
	IsCurrent bool                    `json:"is_current"`
	Status    resolver.WorktreeStatus `json:"status"`
	PRNumber  *int                    `json:"pr_number,omitempty"`
	CIStatus  *string                 `json:"ci_status,omitempty"`
}

// ListArchivedItem represents an archived branch in JSON output.
type ListArchivedItem struct {
	Task   string `json:"task"`
	Type   string `json:"type"`
	Branch string `json:"branch"`
}

// StatusData is the top-level JSON output for the status command.
type StatusData struct {
	Summary   StatusSummary `json:"summary"`
	Worktrees []StatusItem  `json:"worktrees"`
	StaleDays int           `json:"stale_days"`
	Disk      *DiskSummary  `json:"disk,omitempty"`
}

// DiskSummary is the footprint breakdown emitted under --detail.
// MainBytes is a pointer so nil (omitted in JSON) means "could not be
// computed", distinct from a real zero.
type DiskSummary struct {
	TotalBytes     int64  `json:"total_bytes"`
	MainBytes      *int64 `json:"main_bytes,omitempty"`
	WorktreesBytes int64  `json:"worktrees_bytes"`
}

// StatusSummary holds aggregate counts for the status command.
type StatusSummary struct {
	Total  int `json:"total"`
	Dirty  int `json:"dirty"`
	Stale  int `json:"stale"`
	Behind int `json:"behind"`
}

// StatusItem holds per-worktree status in JSON output.
type StatusItem struct {
	Task      string                  `json:"task"`
	Type      string                  `json:"type"`
	Branch    string                  `json:"branch"`
	Status    resolver.WorktreeStatus `json:"status"`
	Age       *StatusAge              `json:"age"`
	SizeBytes *int64                  `json:"size_bytes,omitempty"`
	Recent7D  *int                    `json:"recent_7d,omitempty"`
}

// StatusAge holds last-commit age information.
type StatusAge struct {
	LastCommit string `json:"last_commit"`
	Stale      bool   `json:"stale"`
}

// ExecData is the top-level JSON output for the exec command.
type ExecData struct {
	Command string       `json:"command"`
	Results []ExecResult `json:"results"`
	Success bool         `json:"success"`
}

// ExecResult holds per-worktree execution results in JSON output.
type ExecResult struct {
	Task      string `json:"task"`
	Branch    string `json:"branch"`
	Path      string `json:"path"`
	ExitCode  int    `json:"exit_code"`
	Stdout    string `json:"stdout"`
	Stderr    string `json:"stderr"`
	Error     string `json:"error,omitempty"`
	Cancelled bool   `json:"cancelled,omitempty"`
}

// LogItem represents a worktree's most-recent commit in JSON output.
type LogItem struct {
	Task       string `json:"task"`
	Service    string `json:"service,omitempty"`
	Type       string `json:"type"`
	Branch     string `json:"branch"`
	Path       string `json:"path"`
	LastCommit string `json:"last_commit"`
	Subject    string `json:"subject"`
}

// DepResultJSON mirrors deps.InstallResult for JSON output (add/rename).
type DepResultJSON struct {
	Module string `json:"module"`
	Source string `json:"source,omitempty"`
	Cloned bool   `json:"cloned"`
	Error  string `json:"error,omitempty"`
}

// HookResultJSON mirrors deps.HookResult for JSON output (add/rename).
type HookResultJSON struct {
	Command string `json:"command"`
	Error   string `json:"error,omitempty"`
}

// AddData is the top-level JSON output for the add command.
// Mode discriminates "task", "pr", and "branch-promote".
type AddData struct {
	Mode            string           `json:"mode"`
	Task            string           `json:"task,omitempty"`
	Service         string           `json:"service,omitempty"`
	Branch          string           `json:"branch"`
	Path            string           `json:"path"`
	Source          string           `json:"source,omitempty"`
	PRNumber        *int             `json:"pr_number,omitempty"`
	Copied          []string         `json:"copied"`
	Skipped         []string         `json:"skipped"`
	SkippedSymlinks []string         `json:"skipped_symlinks"`
	Deps            []DepResultJSON  `json:"deps"`
	Hooks           []HookResultJSON `json:"hooks"`
}

// MergeData is the top-level JSON output for the merge command.
type MergeData struct {
	SourceBranch    string   `json:"source_branch"`
	SourcePath      string   `json:"source_path"`
	TargetLabel     string   `json:"target_label"`
	MergingToMain   bool     `json:"merging_to_main"`
	SourceRemoved   bool     `json:"source_removed"`
	WorktreeRemoved bool     `json:"worktree_removed"`
	SourcePrunable  bool     `json:"source_prunable"`
	RemoveError     string   `json:"remove_error,omitempty"`
	RemoteDeleted   bool     `json:"remote_deleted"`
	RemoteError     string   `json:"remote_error,omitempty"`
	DryRun          bool     `json:"dry_run"`
	Steps           []string `json:"steps,omitempty"`
}

// RemoveData is the top-level JSON output for the remove command.
type RemoveData struct {
	Task            string `json:"task"`
	Branch          string `json:"branch"`
	Path            string `json:"path"`
	Prunable        bool   `json:"prunable"`
	WorktreeRemoved bool   `json:"worktree_removed"`
	BranchDeleted   bool   `json:"branch_deleted"`
	KeepBranch      bool   `json:"keep_branch"`
	BranchError     string `json:"branch_error,omitempty"`
	DryRun          bool   `json:"dry_run"`
}

// RenameData is the top-level JSON output for the rename command.
type RenameData struct {
	OldBranch      string           `json:"old_branch"`
	NewBranch      string           `json:"new_branch"`
	OldPath        string           `json:"old_path"`
	NewPath        string           `json:"new_path"`
	Published      bool             `json:"published"`
	PublishError   string           `json:"publish_error,omitempty"`
	RemoteDeleted  bool             `json:"remote_deleted"`
	RemoteError    string           `json:"remote_error,omitempty"`
	RemoteSkipped  bool             `json:"remote_skipped"`
	NoOriginRemote bool             `json:"no_origin_remote"`
	Deps           []DepResultJSON  `json:"deps"`
	Hooks          []HookResultJSON `json:"hooks"`
}

// CleanCandidateJSON describes one worktree eligible for removal in
// list-mode clean (--merged/--stale), before removal has happened.
type CleanCandidateJSON struct {
	Task             string `json:"task"`
	Branch           string `json:"branch"`
	Path             string `json:"path"`
	Prunable         bool   `json:"prunable"`
	WillDeleteRemote bool   `json:"will_delete_remote,omitempty"`
	LastCommit       string `json:"last_commit,omitempty"`
}

// CleanedItemJSON describes the outcome of removing one candidate.
type CleanedItemJSON struct {
	Branch          string `json:"branch"`
	Path            string `json:"path"`
	Prunable        bool   `json:"prunable"`
	WorktreeRemoved bool   `json:"worktree_removed"`
	BranchDeleted   bool   `json:"branch_deleted"`
	RemoteDeleted   bool   `json:"remote_deleted"`
	RemoteError     string `json:"remote_error,omitempty"`
	Error           string `json:"error,omitempty"`
}

// CleanData is the top-level JSON output for the clean command.
// Mode discriminates "prune", "merged", "stale"; the two field groups below are mutually exclusive.
type CleanData struct {
	Mode   string `json:"mode"`
	DryRun bool   `json:"dry_run"`

	// prune mode
	PruneOutput       string   `json:"prune_output,omitempty"`
	NoRemotes         bool     `json:"no_remotes,omitempty"`
	RemotePruned      []string `json:"remote_pruned,omitempty"`
	RemotePruneErrors []string `json:"remote_prune_errors,omitempty"`

	// merged/stale (list) mode
	Candidates   []CleanCandidateJSON `json:"candidates,omitempty"`
	Cleaned      []CleanedItemJSON    `json:"cleaned,omitempty"`
	CleanedCount int                  `json:"cleaned_count,omitempty"`
	Warnings     []string             `json:"warnings,omitempty"`
}

// SyncSummary holds aggregate counters for the sync command.
type SyncSummary struct {
	Synced       int `json:"synced"`
	SkippedDirty int `json:"skipped_dirty"`
	Failed       int `json:"failed"`
	Pushed       int `json:"pushed"`
	PushSkipped  int `json:"push_skipped"`
	PushFailed   int `json:"push_failed"`
}

// SyncWorktreeJSON holds the outcome of syncing a single worktree.
// Planned is set (true) only under --dry-run, where no sync actually ran.
type SyncWorktreeJSON struct {
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
	Planned     bool   `json:"planned,omitempty"`
}

// SyncData is the top-level JSON output for the sync command.
type SyncData struct {
	MainBranch string             `json:"main_branch"`
	Method     string             `json:"method"`
	All        bool               `json:"all"`
	DryRun     bool               `json:"dry_run"`
	Summary    SyncSummary        `json:"summary"`
	Worktrees  []SyncWorktreeJSON `json:"worktrees"`
}

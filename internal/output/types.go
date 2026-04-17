package output

import "github.com/lugassawan/rimba/internal/resolver"

// ListItem represents a worktree entry in JSON output.
type ListItem struct {
	Task      string                  `json:"task"`
	Service   string                  `json:"service,omitempty"`
	Type      string                  `json:"type"`
	Branch    string                  `json:"branch"`
	Path      string                  `json:"path"`
	IsCurrent bool                    `json:"is_current"`
	Status    resolver.WorktreeStatus `json:"status"`
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

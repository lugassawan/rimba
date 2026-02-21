package output

import "github.com/lugassawan/rimba/internal/resolver"

// ListItem represents a worktree entry in JSON output.
type ListItem struct {
	Task      string                  `json:"task"`
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
	Task   string                  `json:"task"`
	Type   string                  `json:"type"`
	Branch string                  `json:"branch"`
	Status resolver.WorktreeStatus `json:"status"`
	Age    *StatusAge              `json:"age"`
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

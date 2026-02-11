package tui

import (
	"github.com/lugassawan/rimba/internal/config"
	"github.com/lugassawan/rimba/internal/conflict"
	"github.com/lugassawan/rimba/internal/git"
	"github.com/lugassawan/rimba/internal/resolver"
)

// view represents the active TUI panel.
type view int

const (
	viewList view = iota
	viewHelp
	viewConflict
	viewExec
)

// worktreeItem holds display data for a single worktree row.
type worktreeItem struct {
	Task      string
	Type      string
	Branch    string
	Path      string
	IsCurrent bool
	Status    resolver.WorktreeStatus
}

// model is the root Bubble Tea model.
type model struct {
	runner  git.Runner
	cfg     *config.Config

	worktrees []worktreeItem
	cursor    int
	view      view

	// Conflict data
	conflictAnalysis *conflict.Analysis

	// Exec data
	execOutput string

	// Status line
	statusMsg string
	err       error

	// Terminal size
	width  int
	height int
}

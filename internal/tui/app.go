package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/lugassawan/rimba/internal/config"
	"github.com/lugassawan/rimba/internal/conflict"
	"github.com/lugassawan/rimba/internal/git"
	"github.com/lugassawan/rimba/internal/operations"
	"github.com/lugassawan/rimba/internal/resolver"
)

// worktreesLoadedMsg is sent when the worktree list has been fetched.
type worktreesLoadedMsg struct {
	items []worktreeItem
	err   error
}

// operationDoneMsg is sent when an async operation completes.
type operationDoneMsg struct {
	msg string
	err error
}

// conflictDoneMsg is sent when a conflict analysis completes.
type conflictDoneMsg struct {
	analysis *conflict.Analysis
	err      error
}

// New creates the root Bubble Tea model.
func New(r git.Runner, cfg *config.Config) model {
	return model{
		runner: r,
		cfg:    cfg,
	}
}

func (m model) Init() tea.Cmd {
	return loadWorktrees(m.runner, m.cfg)
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)

	case worktreesLoadedMsg:
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		m.worktrees = msg.items
		if m.cursor >= len(m.worktrees) {
			m.cursor = max(0, len(m.worktrees)-1)
		}
		m.statusMsg = fmt.Sprintf("%d worktree(s)", len(m.worktrees))
		return m, nil

	case operationDoneMsg:
		if msg.err != nil {
			m.statusMsg = fmt.Sprintf("Error: %v", msg.err)
		} else {
			m.statusMsg = msg.msg
		}
		return m, loadWorktrees(m.runner, m.cfg)

	case conflictDoneMsg:
		if msg.err != nil {
			m.statusMsg = fmt.Sprintf("Conflict check error: %v", msg.err)
		} else {
			m.conflictAnalysis = msg.analysis
			m.view = viewConflict
			m.statusMsg = "Conflict analysis complete"
		}
		return m, nil
	}

	return m, nil
}

func (m model) View() string {
	var b strings.Builder

	switch m.view {
	case viewList:
		b.WriteString(m.renderWorktreeList())
	case viewHelp:
		b.WriteString(m.renderHelp())
	case viewConflict:
		b.WriteString(m.renderConflictView())
	case viewExec:
		b.WriteString(m.renderExecView())
	}

	// Status bar
	if m.err != nil {
		b.WriteString(errorStyle.Render(fmt.Sprintf("\n%v", m.err)))
	} else if m.statusMsg != "" {
		b.WriteString(statusBarStyle.Render(m.statusMsg))
	}

	// Help line
	b.WriteString(helpStyle.Render("  ? help  q quit"))

	return b.String()
}

func (m model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if key.Matches(msg, keys.Quit) {
		return m, tea.Quit
	}
	if key.Matches(msg, keys.Escape) {
		m.view = viewList
		m.err = nil
		return m, nil
	}
	if key.Matches(msg, keys.Help) {
		if m.view == viewHelp {
			m.view = viewList
		} else {
			m.view = viewHelp
		}
		return m, nil
	}
	if m.view == viewList {
		return m.handleListKey(msg)
	}
	return m, nil
}

func (m model) handleListKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, keys.Up):
		if m.cursor > 0 {
			m.cursor--
		}
	case key.Matches(msg, keys.Down):
		if m.cursor < len(m.worktrees)-1 {
			m.cursor++
		}
	case key.Matches(msg, keys.Sync):
		if len(m.worktrees) > 0 {
			wt := m.worktrees[m.cursor]
			m.statusMsg = fmt.Sprintf("Syncing %s...", wt.Task)
			return m, syncWorktreeCmd(m.runner, m.cfg, wt.Task)
		}
	case key.Matches(msg, keys.Remove):
		if len(m.worktrees) > 0 {
			wt := m.worktrees[m.cursor]
			m.statusMsg = fmt.Sprintf("Removing %s...", wt.Task)
			return m, removeWorktreeCmd(m.runner, wt.Task)
		}
	case key.Matches(msg, keys.Merge):
		if len(m.worktrees) > 0 {
			wt := m.worktrees[m.cursor]
			m.statusMsg = fmt.Sprintf("Merging %s...", wt.Task)
			return m, mergeWorktreeCmd(m.runner, m.cfg, wt.Task)
		}
	case key.Matches(msg, keys.Conflict):
		m.statusMsg = "Running conflict check..."
		return m, conflictCheckCmd(m.runner, m.cfg)
	}
	return m, nil
}

// Commands (async operations that return messages)

func loadWorktrees(r git.Runner, cfg *config.Config) tea.Cmd {
	return func() tea.Msg {
		entries, err := git.ListWorktrees(r)
		if err != nil {
			return worktreesLoadedMsg{err: err}
		}

		prefixes := resolver.AllPrefixes()
		var items []worktreeItem
		for _, e := range entries {
			if e.Bare || e.Branch == "" || e.Branch == cfg.DefaultSource {
				continue
			}
			task, matchedPrefix := resolver.TaskFromBranch(e.Branch, prefixes)
			typeLabel := strings.TrimSuffix(matchedPrefix, "/")

			var status resolver.WorktreeStatus
			if dirty, err := git.IsDirty(r, e.Path); err == nil && dirty {
				status.Dirty = true
			}
			ahead, behind, _ := git.AheadBehind(r, e.Path)
			status.Ahead = ahead
			status.Behind = behind

			items = append(items, worktreeItem{
				Task:   task,
				Type:   typeLabel,
				Branch: e.Branch,
				Path:   e.Path,
				Status: status,
			})
		}

		return worktreesLoadedMsg{items: items}
	}
}

func syncWorktreeCmd(r git.Runner, cfg *config.Config, task string) tea.Cmd {
	return func() tea.Msg {
		err := operations.SyncWorktree(r, task, cfg.DefaultSource, false)
		if err != nil {
			return operationDoneMsg{err: err}
		}
		return operationDoneMsg{msg: fmt.Sprintf("Synced %s", task)}
	}
}

func removeWorktreeCmd(r git.Runner, task string) tea.Cmd {
	return func() tea.Msg {
		_, err := operations.RemoveWorktree(r, task, false, false)
		if err != nil {
			return operationDoneMsg{err: err}
		}
		return operationDoneMsg{msg: fmt.Sprintf("Removed %s", task)}
	}
}

func mergeWorktreeCmd(r git.Runner, cfg *config.Config, task string) tea.Cmd {
	return func() tea.Msg {
		_, err := operations.MergeWorktree(r, cfg, task, "", false, true)
		if err != nil {
			return operationDoneMsg{err: err}
		}
		return operationDoneMsg{msg: fmt.Sprintf("Merged %s", task)}
	}
}

func conflictCheckCmd(r git.Runner, cfg *config.Config) tea.Cmd {
	return func() tea.Msg {
		entries, err := git.ListWorktrees(r)
		if err != nil {
			return conflictDoneMsg{err: err}
		}

		var branches []string
		for _, e := range entries {
			if e.Bare || e.Branch == "" || e.Branch == cfg.DefaultSource {
				continue
			}
			branches = append(branches, e.Branch)
		}

		if len(branches) < 2 {
			return conflictDoneMsg{analysis: &conflict.Analysis{}}
		}

		analysis, err := conflict.Analyze(r, cfg.DefaultSource, branches, false)
		return conflictDoneMsg{analysis: analysis, err: err}
	}
}

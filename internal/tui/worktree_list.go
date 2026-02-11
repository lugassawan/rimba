package tui

import (
	"fmt"
	"strings"

	"github.com/lugassawan/rimba/internal/resolver"
)

func (m model) renderWorktreeList() string {
	var b strings.Builder

	b.WriteString(titleStyle.Render("Rimba Worktrees"))
	b.WriteString("\n")

	if len(m.worktrees) == 0 {
		b.WriteString(dimStyle.Render("  No worktrees found. Press 'a' to add one."))
		b.WriteString("\n")
		return b.String()
	}

	for i, wt := range m.worktrees {
		cursor := "  "
		if i == m.cursor {
			cursor = "> "
		}

		task := wt.Task
		if wt.IsCurrent {
			task = "* " + task
		}

		// Style the line based on selection.
		var line string
		typeLabel := wt.Type
		if s, ok := typeStyles[wt.Type]; ok {
			typeLabel = s.Render(wt.Type)
		}

		status := resolver.FormatStatus(wt.Status)
		if wt.Status.Dirty {
			status = warningStyle.Render(status)
		} else if wt.Status.Ahead == 0 && wt.Status.Behind == 0 {
			status = successStyle.Render(status)
		}

		line = fmt.Sprintf("%-20s %-10s %-30s %s", task, typeLabel, wt.Branch, status)

		if i == m.cursor {
			b.WriteString(selectedStyle.Render(cursor + line))
		} else {
			b.WriteString(normalStyle.Render(cursor + line))
		}
		b.WriteString("\n")
	}

	return b.String()
}

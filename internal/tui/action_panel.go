package tui

import "strings"

func (m model) renderHelp() string {
	var b strings.Builder

	b.WriteString(titleStyle.Render("Keyboard Shortcuts"))
	b.WriteString("\n\n")

	shortcuts := []struct{ key, desc string }{
		{"↑/k, ↓/j", "Navigate worktrees"},
		{"a", "Add new worktree"},
		{"d", "Remove selected worktree"},
		{"m", "Merge selected worktree"},
		{"s", "Sync selected worktree"},
		{"o", "Open in editor"},
		{"c", "Run conflict check"},
		{"e", "Exec command across worktrees"},
		{"?", "Toggle help"},
		{"Esc", "Back to list"},
		{"q", "Quit"},
	}

	for _, s := range shortcuts {
		b.WriteString("  ")
		b.WriteString(selectedStyle.Render(s.key))
		b.WriteString("  ")
		b.WriteString(normalStyle.Render(s.desc))
		b.WriteString("\n")
	}

	return b.String()
}

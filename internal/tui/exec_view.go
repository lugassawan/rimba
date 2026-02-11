package tui

import "strings"

func (m model) renderExecView() string {
	var b strings.Builder

	b.WriteString(titleStyle.Render("Exec Results"))
	b.WriteString("\n")

	if m.execOutput == "" {
		b.WriteString(dimStyle.Render("  No exec output yet. Press 'e' from the list to run a command."))
		b.WriteString("\n")
		return b.String()
	}

	b.WriteString(m.execOutput)
	b.WriteString("\n")

	return b.String()
}

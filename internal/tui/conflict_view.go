package tui

import (
	"fmt"
	"strings"
)

func (m model) renderConflictView() string {
	var b strings.Builder

	b.WriteString(titleStyle.Render("Conflict Analysis"))
	b.WriteString("\n")

	if m.conflictAnalysis == nil {
		b.WriteString(dimStyle.Render("  Running analysis..."))
		return b.String()
	}

	if len(m.conflictAnalysis.Overlaps) == 0 {
		b.WriteString(successStyle.Render("  No file overlaps detected."))
		b.WriteString("\n")
		return b.String()
	}

	b.WriteString(headerStyle.Render("File overlaps:"))
	b.WriteString("\n")
	for _, o := range m.conflictAnalysis.Overlaps {
		b.WriteString(fmt.Sprintf("  %s  ← %s\n",
			warningStyle.Render(o.File),
			strings.Join(o.Branches, ", ")))
	}

	if len(m.conflictAnalysis.Pairs) > 0 {
		b.WriteString("\n")
		b.WriteString(headerStyle.Render("Branch pairs:"))
		b.WriteString("\n")
		for _, p := range m.conflictAnalysis.Pairs {
			conflictLabel := ""
			if p.HasConflict {
				conflictLabel = errorStyle.Render(" CONFLICT")
			}
			b.WriteString(fmt.Sprintf("  %s ↔ %s (%d files)%s\n",
				p.BranchA, p.BranchB, len(p.OverlapFiles), conflictLabel))
		}
	}

	return b.String()
}

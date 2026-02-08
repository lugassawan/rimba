package cmd

import (
	"fmt"
	"strings"

	"github.com/lugassawan/rimba/internal/resolver"
	"github.com/lugassawan/rimba/internal/termcolor"
)

// typeColor returns the color for a given worktree type.
func typeColor(t string) termcolor.Color {
	switch t {
	case "feature":
		return termcolor.Cyan
	case "bugfix":
		return termcolor.Yellow
	case "hotfix":
		return termcolor.Red
	case "docs":
		return termcolor.Blue
	case "test":
		return termcolor.Magenta
	case "chore":
		return termcolor.Gray
	default:
		return ""
	}
}

// colorStatus applies colors to each component of a formatted status string.
func colorStatus(p *termcolor.Painter, s resolver.WorktreeStatus) string {
	formatted := resolver.FormatStatus(s)

	if !s.Dirty && s.Ahead == 0 && s.Behind == 0 {
		return p.Paint(formatted, termcolor.Green)
	}

	var parts []string
	if s.Dirty {
		parts = append(parts, p.Paint("[dirty]", termcolor.Yellow))
	}
	if s.Ahead > 0 {
		parts = append(parts, p.Paint(fmt.Sprintf("↑%d", s.Ahead), termcolor.Green))
	}
	if s.Behind > 0 {
		parts = append(parts, p.Paint(fmt.Sprintf("↓%d", s.Behind), termcolor.Red))
	}
	return strings.Join(parts, " ")
}

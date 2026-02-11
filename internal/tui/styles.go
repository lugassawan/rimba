package tui

import "github.com/charmbracelet/lipgloss"

var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("12")). // bright blue
			PaddingBottom(1)

	selectedStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("10")) // bright green

	normalStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("7")) // white

	dimStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("8")) // gray

	errorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("9")) // bright red

	successStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("10")) // bright green

	warningStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("11")) // bright yellow

	headerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("12")).
			PaddingLeft(2)

	helpStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("8")).
			PaddingTop(1)

	statusBarStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("15")).
			Background(lipgloss.Color("8")).
			PaddingLeft(1).
			PaddingRight(1)

	typeStyles = map[string]lipgloss.Style{
		"feature": lipgloss.NewStyle().Foreground(lipgloss.Color("14")), // cyan
		"bugfix":  lipgloss.NewStyle().Foreground(lipgloss.Color("11")), // yellow
		"hotfix":  lipgloss.NewStyle().Foreground(lipgloss.Color("9")),  // red
		"docs":    lipgloss.NewStyle().Foreground(lipgloss.Color("12")), // blue
		"test":    lipgloss.NewStyle().Foreground(lipgloss.Color("13")), // magenta
		"chore":   lipgloss.NewStyle().Foreground(lipgloss.Color("8")),  // gray
	}
)

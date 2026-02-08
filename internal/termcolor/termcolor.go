package termcolor

import (
	"os"
	"strings"
)

// Color represents an ANSI color/style code.
type Color string

const (
	Bold    Color = "\033[1m"
	Red     Color = "\033[31m"
	Green   Color = "\033[32m"
	Yellow  Color = "\033[33m"
	Blue    Color = "\033[34m"
	Magenta Color = "\033[35m"
	Cyan    Color = "\033[36m"
	Gray    Color = "\033[90m"
	Reset   Color = "\033[0m"
)

// Painter applies ANSI colors to strings, respecting NO_COLOR and --no-color.
type Painter struct {
	disabled bool
}

// NewPainter creates a Painter. Colors are disabled if forceDisable is true
// or the NO_COLOR environment variable is set (per no-color.org).
func NewPainter(forceDisable bool) *Painter {
	disabled := forceDisable
	if !disabled {
		if _, ok := os.LookupEnv("NO_COLOR"); ok {
			disabled = true
		}
	}
	return &Painter{disabled: disabled}
}

// Paint wraps s with the given ANSI color codes. Returns s unmodified if
// colors are disabled or no colors are provided.
func (p *Painter) Paint(s string, colors ...Color) string {
	if p.disabled || len(colors) == 0 {
		return s
	}
	var b strings.Builder
	for _, c := range colors {
		b.WriteString(string(c))
	}
	b.WriteString(s)
	b.WriteString(string(Reset))
	return b.String()
}

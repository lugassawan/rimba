package hint

import (
	"fmt"
	"os"

	"github.com/lugassawan/rimba/internal/termcolor"
	"github.com/spf13/cobra"
)

// Option describes a single flag hint shown to the user.
type Option struct {
	Flag        string
	Description string
}

// Hints collects available flag hints for a command and prints them before execution.
type Hints struct {
	cmd     *cobra.Command
	painter *termcolor.Painter
	options []Option
}

// New creates a Hints instance for the given command.
func New(cmd *cobra.Command, p *termcolor.Painter) *Hints {
	return &Hints{cmd: cmd, painter: p}
}

// Add registers a flag hint. The flag name should not include "--" prefix.
func (h *Hints) Add(flag, description string) *Hints {
	h.options = append(h.options, Option{Flag: flag, Description: description})
	return h
}

// Show prints the hint block to stderr, filtering out flags already in use.
// It returns immediately if RIMBA_QUIET is set or no options remain after filtering.
func (h *Hints) Show() {
	if _, ok := os.LookupEnv("RIMBA_QUIET"); ok {
		return
	}

	var remaining []Option
	for _, o := range h.options {
		if f := h.cmd.Flags().Lookup(o.Flag); f != nil && f.Changed {
			continue
		}
		remaining = append(remaining, o)
	}

	if len(remaining) == 0 {
		return
	}

	w := h.cmd.ErrOrStderr()

	fmt.Fprintln(w, h.painter.Paint("Options:", termcolor.Gray))
	for _, o := range remaining {
		line := fmt.Sprintf("  --%-20s %s", o.Flag, o.Description)
		fmt.Fprintln(w, h.painter.Paint(line, termcolor.Gray))
	}
	fmt.Fprintln(w)
}

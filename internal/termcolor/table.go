package termcolor

import (
	"fmt"
	"io"
	"strings"
)

// VisibleLen returns the display width of s, excluding ANSI escape sequences.
func VisibleLen(s string) int {
	n := 0
	inEsc := false
	for _, r := range s {
		if inEsc {
			if (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') {
				inEsc = false
			}
			continue
		}
		if r == '\033' {
			inEsc = true
			continue
		}
		n++
	}
	return n
}

// Table is an ANSI-aware table renderer that pads columns based on visible
// width rather than byte length. This allows colored text to align correctly.
type Table struct {
	rows [][]string
	gap  int
}

// NewTable creates a Table with the given inter-column gap (number of spaces).
func NewTable(gap int) *Table {
	return &Table{gap: gap}
}

// AddRow appends a row of cells to the table.
func (t *Table) AddRow(cells ...string) {
	t.rows = append(t.rows, cells)
}

// Render writes the formatted table to w.
func (t *Table) Render(w io.Writer) {
	if len(t.rows) == 0 {
		return
	}

	widths := t.columnWidths()
	pad := strings.Repeat(" ", t.gap)

	for _, row := range t.rows {
		writeRow(w, row, widths, pad)
	}
}

// columnWidths returns the maximum visible width for each column.
func (t *Table) columnWidths() []int {
	cols := 0
	for _, row := range t.rows {
		if len(row) > cols {
			cols = len(row)
		}
	}

	widths := make([]int, cols)
	for _, row := range t.rows {
		for i, cell := range row {
			if vl := VisibleLen(cell); vl > widths[i] {
				widths[i] = vl
			}
		}
	}
	return widths
}

// writeRow writes a single row with column padding.
func writeRow(w io.Writer, row []string, widths []int, pad string) {
	for i, cell := range row {
		if i > 0 {
			fmt.Fprint(w, pad)
		}
		fmt.Fprint(w, cell)
		// Pad all columns except the last for cleaner output.
		if i < len(row)-1 {
			fmt.Fprint(w, strings.Repeat(" ", widths[i]-VisibleLen(cell)))
		}
	}
	fmt.Fprintln(w)
}

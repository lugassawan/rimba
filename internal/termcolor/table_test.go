package termcolor

import (
	"bytes"
	"strings"
	"testing"
)

func TestVisibleLen(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  int
	}{
		{"plain text", "hello", 5},
		{"empty", "", 0},
		{"with green", "\033[32mhello\033[0m", 5},
		{"with bold+red", "\033[1m\033[31mhello\033[0m", 5},
		{"mixed", "ab\033[32mcd\033[0mef", 6},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := VisibleLen(tt.input)
			if got != tt.want {
				t.Errorf("VisibleLen(%q) = %d, want %d", tt.input, got, tt.want)
			}
		})
	}
}

func TestTableRender(t *testing.T) {
	tbl := NewTable(2)
	tbl.AddRow("NAME", "TYPE", "STATUS")
	tbl.AddRow("auth-flow", "feature", "[dirty]")
	tbl.AddRow("fix", "bugfix", "✓")

	var buf bytes.Buffer
	tbl.Render(&buf)

	lines := strings.Split(strings.TrimRight(buf.String(), "\n"), "\n")
	if len(lines) != 3 {
		t.Fatalf("expected 3 lines, got %d: %q", len(lines), buf.String())
	}

	// Verify column alignment: "NAME" and "auth-flow" should have the same column start
	// for the second column.
	header := lines[0]
	row1 := lines[1]

	headerTypeIdx := strings.Index(header, "TYPE")
	row1TypeIdx := strings.Index(row1, "feature")

	if headerTypeIdx != row1TypeIdx {
		t.Errorf("columns misaligned: TYPE at %d, feature at %d", headerTypeIdx, row1TypeIdx)
	}
}

func TestTableRenderWithANSI(t *testing.T) {
	p := &Painter{disabled: false}

	tbl := NewTable(2)
	tbl.AddRow("NAME", "STATUS")
	tbl.AddRow(p.Paint("short", Green), p.Paint("ok", Green))
	tbl.AddRow("longername", "ok")

	var buf bytes.Buffer
	tbl.Render(&buf)

	lines := strings.Split(strings.TrimRight(buf.String(), "\n"), "\n")
	if len(lines) != 3 {
		t.Fatalf("expected 3 lines, got %d", len(lines))
	}

	// "ok" columns should all start at the same visible position.
	// The ANSI-colored "short" has more bytes but same visible width as "short".
	// "longername" is the widest, so all STATUS columns align to its right edge + gap.
	headerOkIdx := VisibleLen(lines[0][:strings.Index(lines[0], "STATUS")])
	row2OkIdx := VisibleLen(lines[2][:strings.Index(lines[2], "ok")])

	if headerOkIdx != row2OkIdx {
		t.Errorf("ANSI columns misaligned: STATUS at vis %d, row2 ok at vis %d", headerOkIdx, row2OkIdx)
	}
}

func TestTableEmpty(t *testing.T) {
	tbl := NewTable(2)
	var buf bytes.Buffer
	tbl.Render(&buf)
	if buf.Len() != 0 {
		t.Errorf("expected empty output, got %q", buf.String())
	}
}

package cmd

import (
	"strings"
	"testing"
)

func TestPrintBanner(t *testing.T) {
	cmd, buf := newTestCmd()
	printBanner(cmd)

	out := buf.String()
	// The ASCII art contains these patterns
	if !strings.Contains(out, "(_)") {
		t.Errorf("banner output does not contain ASCII art: %q", out)
	}
	if !strings.Contains(out, "v"+Version()) {
		t.Errorf("banner output does not contain version %q: %q", "v"+Version(), out)
	}
}

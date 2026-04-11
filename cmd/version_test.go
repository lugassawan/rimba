package cmd

import (
	"strings"
	"testing"
)

func TestVersion(t *testing.T) {
	got := Version()
	if got != version {
		t.Errorf("Version() = %q, want %q", got, version)
	}
}

func TestVersionCmd(t *testing.T) {
	cmd, buf := newTestCmd()

	// The Run func writes to cmd.OutOrStdout()
	versionCmd.Run(cmd, nil)

	out := buf.String()
	for _, want := range []string{"rimba", version, "commit:", "built:", "os:", "arch:"} {
		if !strings.Contains(out, want) {
			t.Errorf("version output %q does not contain %q", out, want)
		}
	}
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) != 5 {
		t.Errorf("expected 5 lines, got %d: %q", len(lines), out)
	}
}

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
	if !strings.Contains(out, "rimba") {
		t.Errorf("version output %q does not contain 'rimba'", out)
	}
	if !strings.Contains(out, version) {
		t.Errorf("version output %q does not contain version %q", out, version)
	}
}

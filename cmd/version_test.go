package cmd

import (
	"bytes"
	"strings"
	"testing"
)

func TestVersion(t *testing.T) {
	got := Version()
	if got != version {
		t.Errorf("Version() = %q, want %q", got, version)
	}
}

// TestVersionString tests versionString() in isolation from I/O — it is the
// canonical format contract, independent of the subcommand or flag path.
func TestVersionString(t *testing.T) {
	got := versionString()
	for _, want := range []string{"rimba", version, "commit:", "built:", "os:", "arch:", "go:"} {
		if !strings.Contains(got, want) {
			t.Errorf("versionString() %q does not contain %q", got, want)
		}
	}
	lines := strings.Split(strings.TrimSpace(got), "\n")
	if len(lines) != 6 {
		t.Errorf("expected 6 lines, got %d: %q", len(lines), got)
	}
	if !strings.HasSuffix(got, "\n") {
		t.Errorf("versionString() must end with newline; got %q", got)
	}
}

func TestVersionCmd(t *testing.T) {
	cmd, buf := newTestCmd()

	versionCmd.Run(cmd, nil)

	out := buf.String()
	for _, want := range []string{"rimba", version, "commit:", "built:", "os:", "arch:", "go:"} {
		if !strings.Contains(out, want) {
			t.Errorf("version output %q does not contain %q", out, want)
		}
	}
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) != 6 {
		t.Errorf("expected 6 lines, got %d: %q", len(lines), out)
	}
}

func TestVersionFlagOutput(t *testing.T) {
	outBuf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	// rootCmd.Version is set by Execute() in production; mirror that here since
	// the test calls rootCmd.Execute() (cobra method) directly, bypassing cmd.Execute().
	origVersion := rootCmd.Version
	rootCmd.Version = versionString()
	rootCmd.SetOut(outBuf)
	rootCmd.SetErr(errBuf)
	rootCmd.SetArgs([]string{"--version"})
	t.Cleanup(func() {
		rootCmd.Version = origVersion
		rootCmd.SetOut(nil)
		rootCmd.SetErr(nil)
		rootCmd.SetArgs(nil)
	})

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("rootCmd.Execute() error: %v", err)
	}

	if errBuf.Len() > 0 {
		t.Errorf("unexpected stderr: %q", errBuf.String())
	}
	flagOut := outBuf.String()
	want := versionString()
	if flagOut != want {
		t.Errorf("--version output:\ngot  %q\nwant %q", flagOut, want)
	}

	// Byte-identical to subcommand output
	subCmd, subBuf := newTestCmd()
	versionCmd.Run(subCmd, nil)
	if subBuf.String() != flagOut {
		t.Errorf("--version != version subcommand:\nflag %q\nsub  %q", flagOut, subBuf.String())
	}
}

func TestVersionFlagNoShorthand(t *testing.T) {
	if f := rootCmd.Flags().ShorthandLookup("v"); f != nil {
		t.Errorf("expected no -v shorthand, got flag %q", f.Name)
	}
}

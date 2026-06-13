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
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetArgs([]string{"--version"})
	t.Cleanup(func() {
		rootCmd.SetOut(nil)
		rootCmd.SetErr(nil)
		rootCmd.SetArgs(nil)
	})

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("rootCmd.Execute() error: %v", err)
	}

	flagOut := buf.String()
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

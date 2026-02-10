package hint

import (
	"bytes"
	"os"
	"testing"

	"github.com/lugassawan/rimba/internal/termcolor"
	"github.com/spf13/cobra"
)

const (
	testFlagSkipDeps  = "skip-deps"
	testFlagSkipHooks = "skip-hooks"
	testFlagSource    = "source"

	testHintSkipDeps  = "Skip dependency installation"
	testHintSkipHooks = "Skip post-create hooks"
)

func newTestCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "test"}
	cmd.Flags().Bool(testFlagSkipDeps, false, "skip deps")
	cmd.Flags().Bool(testFlagSkipHooks, false, "skip hooks")
	cmd.Flags().StringP(testFlagSource, "s", "", "source branch")
	return cmd
}

func TestShowPrintsAllOptions(t *testing.T) {
	cmd := newTestCmd()
	var buf bytes.Buffer
	cmd.SetErr(&buf)
	p := termcolor.NewPainter(true) // no color for easy assertion

	New(cmd, p).
		Add(testFlagSkipDeps, testHintSkipDeps).
		Add(testFlagSkipHooks, testHintSkipHooks).
		Show()

	out := buf.String()
	if out == "" {
		t.Fatal("expected hint output, got empty")
	}
	assertContains(t, out, "Options:")
	assertContains(t, out, "--"+testFlagSkipDeps)
	assertContains(t, out, "--"+testFlagSkipHooks)
	assertContains(t, out, "Skip dependency installation")
}

func TestShowFiltersUsedFlags(t *testing.T) {
	cmd := newTestCmd()
	_ = cmd.Flags().Set(testFlagSkipDeps, "true")

	var buf bytes.Buffer
	cmd.SetErr(&buf)
	p := termcolor.NewPainter(true)

	New(cmd, p).
		Add(testFlagSkipDeps, testHintSkipDeps).
		Add(testFlagSkipHooks, testHintSkipHooks).
		Show()

	out := buf.String()
	assertNotContains(t, out, "--"+testFlagSkipDeps)
	assertContains(t, out, "--"+testFlagSkipHooks)
}

func TestShowNoOutputWhenAllFiltered(t *testing.T) {
	cmd := newTestCmd()
	_ = cmd.Flags().Set(testFlagSkipDeps, "true")
	_ = cmd.Flags().Set(testFlagSkipHooks, "true")

	var buf bytes.Buffer
	cmd.SetErr(&buf)
	p := termcolor.NewPainter(true)

	New(cmd, p).
		Add(testFlagSkipDeps, testHintSkipDeps).
		Add(testFlagSkipHooks, testHintSkipHooks).
		Show()

	if buf.Len() != 0 {
		t.Errorf("expected no output when all flags used, got: %s", buf.String())
	}
}

func TestShowRespectsRIMBAQUIET(t *testing.T) {
	t.Setenv("RIMBA_QUIET", "1")

	cmd := newTestCmd()
	var buf bytes.Buffer
	cmd.SetErr(&buf)
	p := termcolor.NewPainter(true)

	New(cmd, p).
		Add(testFlagSkipDeps, testHintSkipDeps).
		Show()

	if buf.Len() != 0 {
		t.Errorf("expected no output with RIMBA_QUIET, got: %s", buf.String())
	}
}

func TestShowRespectsRIMBAQUIETEmpty(t *testing.T) {
	t.Setenv("RIMBA_QUIET", "")

	cmd := newTestCmd()
	var buf bytes.Buffer
	cmd.SetErr(&buf)
	p := termcolor.NewPainter(true)

	New(cmd, p).
		Add(testFlagSkipDeps, testHintSkipDeps).
		Show()

	if buf.Len() != 0 {
		t.Errorf("expected no output with RIMBA_QUIET='', got: %s", buf.String())
	}
}

func TestShowNoOptionsIsNoop(t *testing.T) {
	cmd := newTestCmd()
	var buf bytes.Buffer
	cmd.SetErr(&buf)
	p := termcolor.NewPainter(true)

	New(cmd, p).Show()

	if buf.Len() != 0 {
		t.Errorf("expected no output with zero options, got: %s", buf.String())
	}
}

func TestShowTrailingBlankLine(t *testing.T) {
	// Ensure the RIMBA_QUIET env var is NOT set for this test
	os.Unsetenv("RIMBA_QUIET")

	cmd := newTestCmd()
	var buf bytes.Buffer
	cmd.SetErr(&buf)
	p := termcolor.NewPainter(true)

	New(cmd, p).
		Add(testFlagSkipDeps, "Skip deps").
		Show()

	out := buf.String()
	// Should end with two newlines (last option line + trailing blank line)
	if len(out) < 2 || out[len(out)-2:] != "\n\n" {
		t.Errorf("expected trailing blank line, got: %q", out)
	}
}

func assertContains(t *testing.T, s, substr string) {
	t.Helper()
	if !contains(s, substr) {
		t.Errorf("expected %q to contain %q", s, substr)
	}
}

func assertNotContains(t *testing.T, s, substr string) {
	t.Helper()
	if contains(s, substr) {
		t.Errorf("expected %q NOT to contain %q", s, substr)
	}
}

func contains(s, substr string) bool {
	return len(substr) > 0 && len(s) >= len(substr) && containsStr(s, substr)
}

func containsStr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

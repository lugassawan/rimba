package debug

import (
	"io"
	"os"
	"strings"
	"testing"
	"time"
)

func TestStartTimerEnabled(t *testing.T) {
	t.Setenv("RIMBA_DEBUG", "1")

	r, w, _ := os.Pipe()
	origStderr := os.Stderr
	os.Stderr = w
	t.Cleanup(func() { os.Stderr = origStderr })

	stop := StartTimer("test-op")
	stop()

	w.Close()
	output, _ := io.ReadAll(r)

	if !strings.Contains(string(output), "[debug] test-op: start") {
		t.Errorf("expected start line, got %q", output)
	}
	if !strings.Contains(string(output), "[debug] test-op: 0s") {
		t.Errorf("expected duration line, got %q", output)
	}
}

func TestStartTimerDisabled(t *testing.T) {
	t.Setenv("RIMBA_DEBUG", "")
	os.Unsetenv("RIMBA_DEBUG")

	r, w, _ := os.Pipe()
	origStderr := os.Stderr
	os.Stderr = w
	t.Cleanup(func() { os.Stderr = origStderr })

	stop := StartTimer("test-op")
	stop()

	w.Close()
	output, _ := io.ReadAll(r)

	if len(output) != 0 {
		t.Errorf("expected no output when disabled, got %q", output)
	}
}

func TestLogGitTimingEnabledNoDir(t *testing.T) {
	t.Setenv("RIMBA_DEBUG", "1")

	r, w, _ := os.Pipe()
	origStderr := os.Stderr
	os.Stderr = w
	t.Cleanup(func() { os.Stderr = origStderr })

	LogGitTiming("", []string{"status", "-s"}, 0)

	w.Close()
	output, _ := io.ReadAll(r)

	if !strings.Contains(string(output), "[debug] git status -s: 0s") {
		t.Errorf("expected undecorated git label, got %q", output)
	}
}

func TestLogGitTimingEnabledWithDir(t *testing.T) {
	t.Setenv("RIMBA_DEBUG", "1")

	r, w, _ := os.Pipe()
	origStderr := os.Stderr
	os.Stderr = w
	t.Cleanup(func() { os.Stderr = origStderr })

	LogGitTiming("/tmp/worktree", []string{"fetch"}, 5*time.Millisecond)

	w.Close()
	output, _ := io.ReadAll(r)

	if !strings.Contains(string(output), "[debug] git fetch [worktree]: 5ms") {
		t.Errorf("expected dir-suffixed git label, got %q", output)
	}
}

func TestLogGitTimingDisabled(t *testing.T) {
	os.Unsetenv("RIMBA_DEBUG")
	t.Cleanup(func() { os.Unsetenv("RIMBA_DEBUG") })

	r, w, _ := os.Pipe()
	origStderr := os.Stderr
	os.Stderr = w
	t.Cleanup(func() { os.Stderr = origStderr })

	LogGitTiming("/tmp/worktree", []string{"fetch"}, 0)

	w.Close()
	output, _ := io.ReadAll(r)

	if len(output) != 0 {
		t.Errorf("expected no output when disabled, got %q", output)
	}
}

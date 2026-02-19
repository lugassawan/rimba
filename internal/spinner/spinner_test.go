package spinner

import (
	"bytes"
	"os"
	"strings"
	"testing"
	"time"
)

const (
	msgLoading = "Loading..."
	msgStep1   = "Step 1"
	msgStep2   = "Step 2"
	msgCycle1  = "Cycle 1"
	msgCycle2  = "Cycle 2"
)

func requireContains(t *testing.T, got, want string) {
	t.Helper()
	if !strings.Contains(got, want) {
		t.Errorf("expected output to contain %q, got %q", want, got)
	}
}

func TestNonAnimatedStartStop(t *testing.T) {
	var buf bytes.Buffer
	s := New(Options{Writer: &buf})

	s.Start(msgLoading)
	s.Stop()

	requireContains(t, buf.String(), msgLoading)
}

func TestNonAnimatedUpdate(t *testing.T) {
	var buf bytes.Buffer
	s := New(Options{Writer: &buf})

	s.Start(msgStep1)
	s.Update(msgStep2)
	s.Stop()

	got := buf.String()
	requireContains(t, got, msgStep1)
	requireContains(t, got, msgStep2)
}

func TestStopIdempotent(t *testing.T) {
	var buf bytes.Buffer
	s := New(Options{Writer: &buf})

	s.Start(msgLoading)
	s.Stop()
	s.Stop() // should not panic
	s.Stop() // should not panic
}

func TestStopWithoutStart(t *testing.T) {
	var buf bytes.Buffer
	s := New(Options{Writer: &buf})

	s.Stop() // should not panic
}

func TestUpdateWithoutStart(t *testing.T) {
	var buf bytes.Buffer
	s := New(Options{Writer: &buf})

	s.Update("nope") // should not panic

	if buf.Len() != 0 {
		t.Errorf("expected no output, got %q", buf.String())
	}
}

func TestNoColorForcesNonAnimated(t *testing.T) {
	var buf bytes.Buffer
	s := New(Options{Writer: &buf, NoColor: true})

	// Even if writer were a TTY, NoColor forces non-animated
	if s.animated {
		t.Error("expected animated=false when NoColor=true")
	}
}

func TestBufferIsNonTTY(t *testing.T) {
	var buf bytes.Buffer
	s := New(Options{Writer: &buf})

	if s.animated {
		t.Error("expected animated=false for bytes.Buffer (non-TTY)")
	}
}

func TestStartWhileRunningUpdatesMessage(t *testing.T) {
	var buf bytes.Buffer
	s := New(Options{Writer: &buf})

	s.Start("First")
	s.Start("Second") // should act like Update
	s.Stop()

	// In non-animated mode, only "First" gets printed (Start while running just updates msg)
	requireContains(t, buf.String(), "First")
}

func TestMultipleStartStopCycles(t *testing.T) {
	var buf bytes.Buffer
	s := New(Options{Writer: &buf})

	s.Start(msgCycle1)
	s.Stop()
	s.Start(msgCycle2)
	s.Stop()

	got := buf.String()
	requireContains(t, got, msgCycle1)
	requireContains(t, got, msgCycle2)
}

func TestNilWriterDefaultsToStderr(t *testing.T) {
	s := New(Options{})
	if s.w == nil {
		t.Error("expected writer to default to os.Stderr")
	}
}

func TestNonAnimatedOutputHasNewlines(t *testing.T) {
	var buf bytes.Buffer
	s := New(Options{Writer: &buf})

	s.Start("Line 1")
	s.Update("Line 2")
	s.Stop()

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 2 {
		t.Errorf("expected 2 lines, got %d: %q", len(lines), buf.String())
	}
}

func TestIsTTYWithNoColorEnv(t *testing.T) {
	prev, had := os.LookupEnv("NO_COLOR")
	os.Setenv("NO_COLOR", "1")
	defer func() {
		if had {
			os.Setenv("NO_COLOR", prev)
		} else {
			os.Unsetenv("NO_COLOR")
		}
	}()

	// Even with a real file, NO_COLOR should make isTTY return false
	f, err := os.CreateTemp("", "spinner-test")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close(); _ = os.Remove(f.Name()) }()

	if isTTY(f) {
		t.Error("expected isTTY=false when NO_COLOR is set")
	}
}

func TestIsTTYWithNonFileWriter(t *testing.T) {
	prev, had := os.LookupEnv("NO_COLOR")
	os.Unsetenv("NO_COLOR")
	defer func() {
		if had {
			os.Setenv("NO_COLOR", prev)
		}
	}()

	var buf bytes.Buffer
	if isTTY(&buf) {
		t.Error("expected isTTY=false for bytes.Buffer")
	}
}

func TestIsTTYWithFile(t *testing.T) {
	prev, had := os.LookupEnv("NO_COLOR")
	os.Unsetenv("NO_COLOR")
	defer func() {
		if had {
			os.Setenv("NO_COLOR", prev)
		}
	}()

	f, err := os.CreateTemp("", "spinner-test")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close(); _ = os.Remove(f.Name()) }()

	// A regular file is not a TTY
	if isTTY(f) {
		t.Error("expected isTTY=false for regular file")
	}
}

func TestIsTTYStatError(t *testing.T) {
	prev, had := os.LookupEnv("NO_COLOR")
	os.Unsetenv("NO_COLOR")
	defer func() {
		if had {
			os.Setenv("NO_COLOR", prev)
		}
	}()

	f, err := os.CreateTemp("", "spinner-stat-err")
	if err != nil {
		t.Fatal(err)
	}
	name := f.Name()
	// Close file descriptor so Stat() fails
	_ = f.Close()
	_ = os.Remove(name)

	if isTTY(f) {
		t.Error("expected isTTY=false when Stat fails on closed file")
	}
}

func TestAnimatedStartStop(t *testing.T) {
	var buf bytes.Buffer
	s := &Spinner{w: &buf, animated: true}

	s.Start("Animated test")
	time.Sleep(200 * time.Millisecond)
	s.Stop()

	got := buf.String()
	if !strings.Contains(got, "Animated test") {
		t.Errorf("expected output to contain message, got %q", got)
	}
	// clearLine writes \r\033[K
	if !strings.Contains(got, "\033[K") {
		t.Errorf("expected clearLine escape sequence, got %q", got)
	}
}

func TestAnimatedUpdate(t *testing.T) {
	var buf bytes.Buffer
	s := &Spinner{w: &buf, animated: true}

	s.Start("First")
	time.Sleep(100 * time.Millisecond)
	s.Update("Second")
	time.Sleep(200 * time.Millisecond)
	s.Stop()

	got := buf.String()
	if !strings.Contains(got, "Second") {
		t.Errorf("expected output to contain updated message, got %q", got)
	}
}

func TestAnimatedDoubleStop(t *testing.T) {
	var buf bytes.Buffer
	s := &Spinner{w: &buf, animated: true}

	s.Start("Test")
	time.Sleep(100 * time.Millisecond)
	s.Stop()
	s.Stop() // should not panic
}

func TestAnimatedStartWhileRunning(t *testing.T) {
	var buf bytes.Buffer
	s := &Spinner{w: &buf, animated: true}

	s.Start("First")
	s.Start("Second") // should update message, not start new goroutine
	time.Sleep(200 * time.Millisecond)
	s.Stop()

	got := buf.String()
	if !strings.Contains(got, "Second") {
		t.Errorf("expected output to contain updated message, got %q", got)
	}
}

func TestConcurrentStopSafe(t *testing.T) {
	var buf bytes.Buffer
	s := New(Options{Writer: &buf})

	s.Start("Concurrent")

	done := make(chan struct{})
	go func() {
		s.Stop()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("concurrent Stop timed out")
	}

	s.Stop() // idempotent
}

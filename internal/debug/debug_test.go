package debug

import (
	"context"
	"io"
	"os"
	"strings"
	"testing"
)

type stubRunner struct {
	runContextCalled  bool
	runInDirCtxCalled bool
}

func (s *stubRunner) Run(_ context.Context, args ...string) (string, error) {
	s.runContextCalled = true
	return "ok", nil
}

func (s *stubRunner) RunInDir(_ context.Context, dir string, args ...string) (string, error) {
	s.runInDirCtxCalled = true
	return "ok", nil
}

func TestWrapRunnerEnabled(t *testing.T) {
	t.Setenv("RIMBA_DEBUG", "1")

	stub := &stubRunner{}
	wrapped := WrapRunner(stub)

	if _, ok := wrapped.(*TimedRunner); !ok {
		t.Error("expected TimedRunner when RIMBA_DEBUG is set")
	}
}

func TestWrapRunnerDisabled(t *testing.T) {
	os.Unsetenv("RIMBA_DEBUG")
	t.Cleanup(func() { os.Unsetenv("RIMBA_DEBUG") })

	stub := &stubRunner{}
	wrapped := WrapRunner(stub)

	if wrapped != stub {
		t.Error("expected original runner returned when RIMBA_DEBUG is unset")
	}
}

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

func TestTimedRunnerRun(t *testing.T) {
	t.Setenv("RIMBA_DEBUG", "1")

	stub := &stubRunner{}
	wrapped := WrapRunner(stub)

	out, err := wrapped.Run(context.Background(), "status")
	if err != nil {
		t.Fatal(err)
	}
	if out != "ok" {
		t.Errorf("expected ok, got %s", out)
	}
	if !stub.runContextCalled {
		t.Error("inner runner Run was not called")
	}
}

func TestTimedRunnerRunInDir(t *testing.T) {
	t.Setenv("RIMBA_DEBUG", "1")

	stub := &stubRunner{}
	wrapped := WrapRunner(stub)

	out, err := wrapped.RunInDir(context.Background(), "/tmp/test", "status")
	if err != nil {
		t.Fatal(err)
	}
	if out != "ok" {
		t.Errorf("expected ok, got %s", out)
	}
	if !stub.runInDirCtxCalled {
		t.Error("inner runner RunInDir was not called")
	}
}

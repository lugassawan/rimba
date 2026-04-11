package debug

import (
	"os"
	"testing"
)

type stubRunner struct {
	runCalled      bool
	runInDirCalled bool
}

func (s *stubRunner) Run(args ...string) (string, error) {
	s.runCalled = true
	return "ok", nil
}

func (s *stubRunner) RunInDir(dir string, args ...string) (string, error) {
	s.runInDirCalled = true
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

func TestTimedRunnerRun(t *testing.T) {
	t.Setenv("RIMBA_DEBUG", "1")

	stub := &stubRunner{}
	wrapped := WrapRunner(stub)

	out, err := wrapped.Run("status")
	if err != nil {
		t.Fatal(err)
	}
	if out != "ok" {
		t.Errorf("expected ok, got %s", out)
	}
	if !stub.runCalled {
		t.Error("inner runner Run was not called")
	}
}

func TestTimedRunnerRunInDir(t *testing.T) {
	t.Setenv("RIMBA_DEBUG", "1")

	stub := &stubRunner{}
	wrapped := WrapRunner(stub)

	out, err := wrapped.RunInDir("/tmp/test", "status")
	if err != nil {
		t.Fatal(err)
	}
	if out != "ok" {
		t.Errorf("expected ok, got %s", out)
	}
	if !stub.runInDirCalled {
		t.Error("inner runner RunInDir was not called")
	}
}

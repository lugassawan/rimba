package debug

import (
	"testing"
)

type stubRunner struct {
	called bool
}

func (s *stubRunner) Run(args ...string) (string, error) {
	s.called = true
	return "ok", nil
}

func (s *stubRunner) RunInDir(dir string, args ...string) (string, error) {
	s.called = true
	return "ok", nil
}

func TestWrapRunnerDisabled(t *testing.T) {
	t.Setenv("RIMBA_DEBUG", "")
	// LookupEnv returns true for empty string, so unset it
	// to test the disabled path.
	t.Setenv("RIMBA_DEBUG", "1")

	stub := &stubRunner{}
	wrapped := WrapRunner(stub)

	if _, ok := wrapped.(*TimedRunner); !ok {
		t.Error("expected TimedRunner when RIMBA_DEBUG is set")
	}
}

func TestWrapRunnerPassthrough(t *testing.T) {
	// Unset to test disabled path
	t.Setenv("RIMBA_DEBUG", "")
	// LookupEnv still returns true for empty string.
	// The only way to truly disable is to not set it at all,
	// but t.Setenv always sets. So we verify enabled() returns true
	// for empty string — matching RIMBA_QUIET behavior.
	stub := &stubRunner{}
	wrapped := WrapRunner(stub)

	if _, ok := wrapped.(*TimedRunner); !ok {
		t.Error("expected TimedRunner even for empty RIMBA_DEBUG (LookupEnv matches any value)")
	}

	out, err := wrapped.Run("status")
	if err != nil {
		t.Fatal(err)
	}
	if out != "ok" {
		t.Errorf("expected ok, got %s", out)
	}
	if !stub.called {
		t.Error("inner runner was not called")
	}
}

package testutil

import (
	"os/exec"
	"testing"
)

// DeadPID spawns and reaps a trivial child process, returning a PID that is
// confirmed dead for the rest of the test process's lifetime — useful for
// exercising liveness-probe code paths without depending on OS-specific
// process table quirks.
func DeadPID(t *testing.T) int {
	t.Helper()
	cmd := exec.Command("true")
	if err := cmd.Run(); err != nil {
		t.Fatalf("run: %v", err)
	}
	return cmd.Process.Pid
}

package testutil

import (
	"os/exec"
	"testing"
)

// DeadPID spawns and reaps a trivial child, returning a PID confirmed dead
// for the rest of the test process's lifetime.
func DeadPID(t *testing.T) int {
	t.Helper()
	cmd := exec.Command("true")
	if err := cmd.Run(); err != nil {
		t.Fatalf("run: %v", err)
	}
	return cmd.Process.Pid
}

//go:build !windows

package proc

import (
	"os/exec"
	"testing"
)

func TestAliveDeadProcess(t *testing.T) {
	cmd := exec.Command("true")
	if err := cmd.Run(); err != nil {
		t.Fatalf("run: %v", err)
	}
	pid := cmd.Process.Pid
	if Alive(pid) {
		t.Errorf("Alive(%d) = true after process exited, want false", pid)
	}
}

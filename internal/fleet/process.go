package fleet

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"
)

// Process wraps an OS process for fleet task management.
type Process struct {
	PID  int
	Cmd  *exec.Cmd
	Done chan struct{}
}

// IsAlive checks if a process with the given PID is still running.
func IsAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	// On Unix, FindProcess always succeeds. Send signal 0 to check.
	err = proc.Signal(syscall.Signal(0))
	return err == nil
}

// StopProcess attempts to terminate a process by PID.
func StopProcess(pid int) error {
	if pid <= 0 {
		return fmt.Errorf("invalid PID: %d", pid)
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return fmt.Errorf("process not found: %w", err)
	}
	return proc.Signal(syscall.SIGTERM)
}

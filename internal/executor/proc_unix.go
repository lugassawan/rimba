//go:build unix

package executor

import (
	"os/exec"
	"syscall"
	"time"
)

// configureProcessGroup SIGTERMs cmd's whole process group on cancel, then
// SIGKILLs it if grace elapses before cleanup() (called after cmd.Run()) fires.
func configureProcessGroup(cmd *exec.Cmd, grace time.Duration) (cleanup func()) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	exited := make(chan struct{})
	cmd.Cancel = func() error {
		pgid := cmd.Process.Pid // Setpgid(0,0) → PID == PGID
		_ = syscall.Kill(-pgid, syscall.SIGTERM)
		go func() {
			select {
			case <-exited: // avoid signalling a possibly-recycled PGID
			case <-time.After(grace):
				_ = syscall.Kill(-pgid, syscall.SIGKILL)
			}
		}()
		return nil // nil → exec still runs WaitDelay as a backstop
	}
	cmd.WaitDelay = grace + time.Second
	return func() { close(exited) }
}

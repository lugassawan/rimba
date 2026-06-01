//go:build unix

package deps

import (
	"os/exec"
	"syscall"
)

// configureProcessGroup puts the subprocess in its own process group so that
// context cancellation kills the shell and all children it may have forked
// (e.g. `sh -c "sleep 30"` or npm spawning node children).
func configureProcessGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		if cmd.Process != nil {
			_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		}
		return nil
	}
}

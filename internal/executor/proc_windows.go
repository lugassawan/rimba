//go:build windows

package executor

import (
	"os/exec"
	"time"
)

// No POSIX process groups on Windows; WaitDelay still bounds the pipe-drain
// hang from a grandchild inheriting ShellRunner's stdout/stderr pipes.
func configureProcessGroup(cmd *exec.Cmd, grace time.Duration) (cleanup func()) {
	cmd.WaitDelay = grace + time.Second
	return func() {}
}

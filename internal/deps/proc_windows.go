//go:build windows

package deps

import "os/exec"

// configureProcessGroup is a no-op on Windows, which has no POSIX process
// groups. exec.CommandContext's default cancellation (Process.Kill) is used.
func configureProcessGroup(cmd *exec.Cmd) {}

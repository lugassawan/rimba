// Package gh detects and authenticates the GitHub CLI.
package gh

import "os/exec"

// IsAvailable reports whether `gh` is on PATH.
func IsAvailable() bool {
	_, err := exec.LookPath("gh")
	return err == nil
}

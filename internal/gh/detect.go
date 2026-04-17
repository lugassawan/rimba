// Package gh provides shared helpers for detecting and authenticating the
// GitHub CLI (`gh`), so downstream features share one detection path and
// uniform error messages.
package gh

import "os/exec"

// IsAvailable reports whether the `gh` binary is on PATH.
func IsAvailable() bool {
	_, err := exec.LookPath("gh")
	return err == nil
}

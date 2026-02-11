//go:build darwin

package updater

import (
	"fmt"
	"os/exec"
)

// PrepareBinary removes the macOS quarantine attribute and ad-hoc code-signs
// the binary so it can execute on Apple Silicon without being killed.
func PrepareBinary(path string) error {
	// Best-effort: remove quarantine attribute (may not exist).
	_ = exec.Command("xattr", "-d", "com.apple.quarantine", path).Run() //nolint:gosec // path from controlled temp dir

	// Ad-hoc sign — fatal on failure.
	if err := exec.Command("codesign", "--sign", "-", "--force", path).Run(); err != nil { //nolint:gosec // path from controlled temp dir
		return fmt.Errorf("code-signing binary: %w", err)
	}

	return nil
}

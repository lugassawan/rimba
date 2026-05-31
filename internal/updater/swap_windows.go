//go:build windows

package updater

import (
	"fmt"

	"github.com/lugassawan/rimba/internal/errhint"
)

// swapBinary is not yet implemented on Windows.
// The rename-aside mechanism required to replace a running .exe is tracked in
// https://github.com/lugassawan/rimba/issues/234
func swapBinary(_, _ string) error {
	return errhint.WithFix(
		fmt.Errorf("self-update replace is not yet supported on Windows"),
		"download the latest release from https://github.com/lugassawan/rimba/releases; "+
			"Windows self-replace support is tracked in https://github.com/lugassawan/rimba/issues/234",
	)
}

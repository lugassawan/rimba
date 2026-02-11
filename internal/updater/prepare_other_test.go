//go:build !darwin

package updater

import "testing"

func TestPrepareBinaryNoOp(t *testing.T) {
	// On non-darwin platforms, PrepareBinary should always return nil.
	if err := PrepareBinary("/any/path"); err != nil {
		t.Errorf("PrepareBinary() = %v, want nil", err)
	}
}

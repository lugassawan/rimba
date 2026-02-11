//go:build !darwin

package updater

// PrepareBinary is a no-op on non-darwin platforms.
func PrepareBinary(_ string) error {
	return nil
}

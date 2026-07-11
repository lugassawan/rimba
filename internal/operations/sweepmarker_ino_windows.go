//go:build windows

package operations

// dirIno has no portable identity signal via os.Stat on Windows; the
// identity guard is skipped there and the recorded path is trusted as-is.
func dirIno(string) (ino uint64, ok bool) {
	return 0, false
}

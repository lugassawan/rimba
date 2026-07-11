//go:build windows

package proc

// Alive always reports true on Windows: there's no signal-0 probe here, so
// callers must rely on an age-based fallback instead.
func Alive(int) bool {
	return true
}

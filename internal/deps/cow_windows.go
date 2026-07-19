//go:build windows

package deps

// sameDevice is unknown on Windows (os.FileInfo.Sys() carries no comparable
// device id here). ok=false makes the CoW-eligibility gate treat this as
// ineligible — Windows attempts no CoW clone today, so this preserves that.
func sameDevice(a, b string) (same bool, ok bool) {
	return false, false
}

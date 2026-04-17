package resolver

import (
	"fmt"
	"strconv"
)

// FormatBytes renders n as a 1024-base size, e.g. "1.8GB" or "15MB".
// Values under 10 in their unit get one decimal; larger values round to
// whole numbers. n <= 0 returns "0B".
func FormatBytes(n int64) string {
	if n <= 0 {
		return "0B"
	}

	const unit = 1024
	if n < unit {
		return strconv.FormatInt(n, 10) + "B"
	}

	units := []string{"KB", "MB", "GB", "TB", "PB", "EB"}
	f := float64(n) / unit
	idx := 0
	for f >= unit && idx < len(units)-1 {
		f /= unit
		idx++
	}
	if f < 10 {
		return fmt.Sprintf("%.1f%s", f, units[idx])
	}
	return fmt.Sprintf("%.0f%s", f, units[idx])
}

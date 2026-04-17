package resolver

import (
	"fmt"
	"strconv"
)

// FormatBytes returns a human-readable size using 1024-base units (B, KB,
// MB, GB, TB). One decimal place is shown for values under 10 in their unit
// (e.g. "1.8GB"); whole numbers for larger values ("15MB"). Negative input
// is clamped to zero.
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

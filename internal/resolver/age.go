package resolver

import (
	"strconv"
	"time"
)

// FormatAge returns a human-readable age string relative to now.
func FormatAge(t time.Time) string {
	return FormatAgeSince(t, time.Now())
}

// FormatAgeSince returns a human-readable age string relative to the given now time.
// Returns "just now" for <1 minute, "Xh ago" for <24h, "Xd ago" for <14d, "Xw ago" otherwise.
func FormatAgeSince(t, now time.Time) string {
	d := max(now.Sub(t), 0)

	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return formatUnit(int(d.Minutes()), "m")
	case d < 24*time.Hour:
		return formatUnit(int(d.Hours()), "h")
	case d < 14*24*time.Hour:
		return formatUnit(int(d.Hours()/24), "d")
	default:
		return formatUnit(int(d.Hours()/(24*7)), "w")
	}
}

func formatUnit(n int, unit string) string {
	return strconv.Itoa(n) + unit + " ago"
}

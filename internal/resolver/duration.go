package resolver

import (
	"fmt"
	"strconv"
	"time"
)

// ParseDuration parses human-friendly duration strings like "7d", "2w", "3h".
func ParseDuration(s string) (time.Duration, error) {
	if len(s) < 2 {
		return 0, fmt.Errorf("duration too short: %q", s)
	}

	numStr := s[:len(s)-1]
	unit := s[len(s)-1]

	n, err := strconv.Atoi(numStr)
	if err != nil {
		return 0, fmt.Errorf("invalid number in duration %q: %w", s, err)
	}
	if n <= 0 {
		return 0, fmt.Errorf("duration must be positive: %q", s)
	}

	switch unit {
	case 'h':
		return time.Duration(n) * time.Hour, nil
	case 'd':
		return time.Duration(n) * 24 * time.Hour, nil
	case 'w':
		return time.Duration(n) * 7 * 24 * time.Hour, nil
	default:
		return 0, fmt.Errorf("unknown duration unit %q in %q (use h, d, or w)", string(unit), s)
	}
}

package observability

import (
	"strings"
	"testing"
)

func TestTruncate(t *testing.T) {
	tests := []struct {
		name  string
		s     string
		limit int
	}{
		{name: "empty", s: "", limit: 10},
		{name: "under limit", s: "hello", limit: 10},
		{name: "exactly at limit", s: "0123456789", limit: 10},
		{name: "over limit", s: strings.Repeat("a", 20), limit: 10},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := truncate(tt.s, tt.limit)
			if len(tt.s) <= tt.limit {
				if got != tt.s {
					t.Errorf("truncate(%q, %d) = %q, want unchanged %q", tt.s, tt.limit, got, tt.s)
				}
				return
			}
			if !strings.HasSuffix(got, "...(truncated)") {
				t.Errorf("truncate(%q, %d) = %q, want suffix %q", tt.s, tt.limit, got, "...(truncated)")
			}
			if !strings.HasPrefix(got, tt.s[:tt.limit]) {
				t.Errorf("truncate(%q, %d) = %q, want prefix %q", tt.s, tt.limit, got, tt.s[:tt.limit])
			}
		})
	}
}

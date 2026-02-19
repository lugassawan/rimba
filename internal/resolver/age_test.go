package resolver_test

import (
	"testing"
	"time"

	"github.com/lugassawan/rimba/internal/resolver"
)

func TestFormatAgeSince(t *testing.T) {
	now := time.Date(2025, 6, 15, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name string
		t    time.Time
		want string
	}{
		{"just now", now.Add(-30 * time.Second), "just now"},
		{"minutes ago", now.Add(-5 * time.Minute), "5m ago"},
		{"one minute ago", now.Add(-1 * time.Minute), "1m ago"},
		{"hours ago", now.Add(-5 * time.Hour), "5h ago"},
		{"one hour ago", now.Add(-1 * time.Hour), "1h ago"},
		{"days ago", now.Add(-3 * 24 * time.Hour), "3d ago"},
		{"one day ago", now.Add(-1 * 24 * time.Hour), "1d ago"},
		{"weeks ago", now.Add(-21 * 24 * time.Hour), "3w ago"},
		{"two weeks ago", now.Add(-14 * 24 * time.Hour), "2w ago"},
		{"future is just now", now.Add(5 * time.Minute), "just now"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resolver.FormatAgeSince(tt.t, now)
			if got != tt.want {
				t.Errorf("FormatAgeSince(%v, %v) = %q, want %q", tt.t, now, got, tt.want)
			}
		})
	}
}

func TestFormatAge(t *testing.T) {
	// FormatAge delegates to FormatAgeSince with time.Now(); just verify it returns something sensible.
	got := resolver.FormatAge(time.Now().Add(-2 * time.Hour))
	if got != "2h ago" {
		t.Errorf("FormatAge(2h ago) = %q, want %q", got, "2h ago")
	}
}

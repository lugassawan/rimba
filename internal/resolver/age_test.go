package resolver_test

import (
	"testing"
	"time"

	"github.com/lugassawan/rimba/internal/resolver"
	"github.com/lugassawan/rimba/internal/termcolor"
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

func TestAgeColor(t *testing.T) {
	tests := []struct {
		name string
		age  time.Duration
		want termcolor.Color
	}{
		{"recent", 1 * time.Hour, termcolor.Green},
		{"two days", 2 * 24 * time.Hour, termcolor.Green},
		{"three days", 3 * 24 * time.Hour, termcolor.Yellow},
		{"one week", 7 * 24 * time.Hour, termcolor.Yellow},
		{"two weeks", 14 * 24 * time.Hour, termcolor.Red},
		{"one month", 30 * 24 * time.Hour, termcolor.Red},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			commitTime := time.Now().Add(-tt.age)
			got := resolver.AgeColor(commitTime)
			if got != tt.want {
				t.Errorf("AgeColor(-%v) = %q, want %q", tt.age, got, tt.want)
			}
		})
	}
}

func TestAgeColorValues(t *testing.T) {
	tests := []struct {
		name string
		age  time.Duration
		want termcolor.Color
	}{
		{"recent", 1 * time.Hour, termcolor.Green},
		{"few days", 5 * 24 * time.Hour, termcolor.Yellow},
		{"old", 30 * 24 * time.Hour, termcolor.Red},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resolver.AgeColor(time.Now().Add(-tt.age))
			if got != tt.want {
				t.Errorf("AgeColor(-%v) = %q, want %q", tt.age, got, tt.want)
			}
		})
	}
}

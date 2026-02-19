package cmd

import (
	"testing"

	"github.com/lugassawan/rimba/internal/resolver"
	"github.com/lugassawan/rimba/internal/termcolor"
)

func TestTypeColor(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  termcolor.Color
	}{
		{"feature", "feature", termcolor.Cyan},
		{"bugfix", "bugfix", termcolor.Yellow},
		{"hotfix", "hotfix", termcolor.Red},
		{"docs", "docs", termcolor.Blue},
		{"test", "test", termcolor.Magenta},
		{"chore", "chore", termcolor.Gray},
		{"unknown", "other", termcolor.Color("")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := typeColor(tt.input)
			if got != tt.want {
				t.Errorf("typeColor(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestColorStatus(t *testing.T) {
	p := termcolor.NewPainter(true) // no-color mode for predictable output

	tests := []struct {
		name   string
		status resolver.WorktreeStatus
		want   string
	}{
		{
			name:   "clean",
			status: resolver.WorktreeStatus{},
			want:   "✓",
		},
		{
			name:   "dirty only",
			status: resolver.WorktreeStatus{Dirty: true},
			want:   "[dirty]",
		},
		{
			name:   "ahead only",
			status: resolver.WorktreeStatus{Ahead: 3},
			want:   "↑3",
		},
		{
			name:   "behind only",
			status: resolver.WorktreeStatus{Behind: 2},
			want:   "↓2",
		},
		{
			name:   "dirty and ahead and behind",
			status: resolver.WorktreeStatus{Dirty: true, Ahead: 2, Behind: 1},
			want:   "[dirty] ↑2 ↓1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := colorStatus(p, tt.status)
			if got != tt.want {
				t.Errorf("colorStatus(%+v) = %q, want %q", tt.status, got, tt.want)
			}
		})
	}
}

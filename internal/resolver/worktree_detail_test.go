package resolver_test

import (
	"testing"

	"github.com/lugassawan/rimba/internal/resolver"
)

func TestFormatStatus(t *testing.T) {
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
			name:   "ahead and behind",
			status: resolver.WorktreeStatus{Ahead: 2, Behind: 1},
			want:   "↑2 ↓1",
		},
		{
			name:   "dirty and ahead",
			status: resolver.WorktreeStatus{Dirty: true, Ahead: 5},
			want:   "[dirty] ↑5",
		},
		{
			name:   "dirty and behind",
			status: resolver.WorktreeStatus{Dirty: true, Behind: 3},
			want:   "[dirty] ↓3",
		},
		{
			name:   "dirty ahead and behind",
			status: resolver.WorktreeStatus{Dirty: true, Ahead: 2, Behind: 1},
			want:   "[dirty] ↑2 ↓1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resolver.FormatStatus(tt.status)
			if got != tt.want {
				t.Errorf("FormatStatus(%+v) = %q, want %q", tt.status, got, tt.want)
			}
		})
	}
}

func TestNewWorktreeDetailWithStatus(t *testing.T) {
	prefixes := resolver.AllPrefixes()
	status := resolver.WorktreeStatus{Dirty: true, Ahead: 1}

	d := resolver.NewWorktreeDetail("feature/my-task", prefixes, "feature-my-task", status, true)

	if d.Task != "my-task" {
		t.Errorf("Task = %q, want %q", d.Task, "my-task")
	}
	if d.Type != "feature" {
		t.Errorf("Type = %q, want %q", d.Type, "feature")
	}
	if !d.IsCurrent {
		t.Error("IsCurrent = false, want true")
	}
	if !d.Status.Dirty {
		t.Error("Status.Dirty = false, want true")
	}
	if d.Status.Ahead != 1 {
		t.Errorf("Status.Ahead = %d, want 1", d.Status.Ahead)
	}
}

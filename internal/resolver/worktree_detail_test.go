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

func TestSortDetailsByTask(t *testing.T) {
	details := []resolver.WorktreeDetail{
		{Task: "charlie"},
		{Task: "alpha"},
		{Task: "bravo"},
	}

	resolver.SortDetailsByTask(details)

	want := []string{"alpha", "bravo", "charlie"}
	for i, w := range want {
		if details[i].Task != w {
			t.Errorf("details[%d].Task = %q, want %q", i, details[i].Task, w)
		}
	}
}

func TestSortDetailsByTaskEmpty(t *testing.T) {
	var details []resolver.WorktreeDetail
	resolver.SortDetailsByTask(details) // should not panic
}

func TestNewWorktreeDetailWithService(t *testing.T) {
	prefixes := resolver.AllPrefixes()
	d := resolver.NewWorktreeDetail("auth-api/feature/login", prefixes, "/wt/auth-api-feature-login", resolver.WorktreeStatus{}, false)

	if d.Service != "auth-api" {
		t.Errorf("Service = %q, want %q", d.Service, "auth-api")
	}
	if d.Task != "login" {
		t.Errorf("Task = %q, want %q", d.Task, "login")
	}
	if d.Type != "feature" {
		t.Errorf("Type = %q, want %q", d.Type, "feature")
	}
}

func TestHasService(t *testing.T) {
	t.Run("with service", func(t *testing.T) {
		details := []resolver.WorktreeDetail{
			{Task: "login", Service: ""},
			{Task: "signup", Service: "auth-api"},
		}
		if !resolver.HasService(details) {
			t.Error("expected true when a detail has service")
		}
	})

	t.Run("without service", func(t *testing.T) {
		details := []resolver.WorktreeDetail{
			{Task: "login"},
			{Task: "signup"},
		}
		if resolver.HasService(details) {
			t.Error("expected false when no detail has service")
		}
	})

	t.Run("empty", func(t *testing.T) {
		if resolver.HasService(nil) {
			t.Error("expected false for nil slice")
		}
	})
}

func TestFilterByService(t *testing.T) {
	details := []resolver.WorktreeDetail{
		{Task: "login", Service: "auth-api"},
		{Task: "signup", Service: "web-app"},
		{Task: "plain", Service: ""},
	}

	t.Run("filter by auth-api", func(t *testing.T) {
		got := resolver.FilterByService(details, "auth-api")
		if len(got) != 1 || got[0].Task != "login" {
			t.Errorf("expected [login], got %v", got)
		}
	})

	t.Run("filter by web-app", func(t *testing.T) {
		got := resolver.FilterByService(details, "web-app")
		if len(got) != 1 || got[0].Task != "signup" {
			t.Errorf("expected [signup], got %v", got)
		}
	})

	t.Run("no match", func(t *testing.T) {
		got := resolver.FilterByService(details, "unknown")
		if len(got) != 0 {
			t.Errorf("expected empty, got %v", got)
		}
	})

	t.Run("empty service returns original", func(t *testing.T) {
		got := resolver.FilterByService(details, "")
		if len(got) != len(details) {
			t.Errorf("expected %d items, got %d", len(details), len(got))
		}
	})
}

func TestNewWorktreeDetailUnknownPrefix(t *testing.T) {
	const customBranch = "custom-branch"
	prefixes := resolver.AllPrefixes()
	d := resolver.NewWorktreeDetail(customBranch, prefixes, "/path", resolver.WorktreeStatus{}, false)

	if d.Task != customBranch {
		t.Errorf("Task = %q, want %q", d.Task, customBranch)
	}
	if d.Type != "" {
		t.Errorf("Type = %q, want empty string", d.Type)
	}
}

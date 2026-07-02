package operations

import (
	"context"
	"testing"

	"github.com/lugassawan/rimba/internal/resolver"
)

const (
	taskLogin  = "login"
	taskSignup = "signup"
	taskLogout = "logout"
)

func TestCollectWorktreeStatusClean(t *testing.T) {
	r := &mockRunner{
		run:      func(_ ...string) (string, error) { return "", nil },
		runInDir: noopRunInDir,
	}
	status := CollectWorktreeStatus(context.Background(), r, "/wt/feature-login")
	if status.Dirty {
		t.Error("expected Dirty=false for clean worktree")
	}
	if status.Ahead != 0 || status.Behind != 0 {
		t.Errorf("expected Ahead=0 Behind=0, got Ahead=%d Behind=%d", status.Ahead, status.Behind)
	}
}

func TestCollectWorktreeStatusDirty(t *testing.T) {
	r := &mockRunner{
		run: func(_ ...string) (string, error) { return "", nil },
		runInDir: func(_ string, args ...string) (string, error) {
			if len(args) >= 1 && args[0] == gitCmdStatus {
				return dirtyStatusFixture, nil
			}
			return "", nil
		},
	}
	status := CollectWorktreeStatus(context.Background(), r, "/wt/feature-login")
	if !status.Dirty {
		t.Error("expected Dirty=true")
	}
}

func TestCollectWorktreeStatusAheadBehind(t *testing.T) {
	r := &mockRunner{
		run: func(_ ...string) (string, error) { return "", nil },
		runInDir: func(_ string, args ...string) (string, error) {
			if len(args) >= 1 && args[0] == "rev-list" {
				return "2\t3", nil
			}
			return "", nil
		},
	}
	status := CollectWorktreeStatus(context.Background(), r, "/wt/feature-login")
	if status.Ahead != 3 {
		t.Errorf("Ahead = %d, want 3", status.Ahead)
	}
	if status.Behind != 2 {
		t.Errorf("Behind = %d, want 2", status.Behind)
	}
}

func TestCollectWorktreeStatusIsDirtyError(t *testing.T) {
	r := &mockRunner{
		run: func(_ ...string) (string, error) { return "", nil },
		runInDir: func(_ string, _ ...string) (string, error) {
			return "", errGitFailed
		},
	}
	status := CollectWorktreeStatus(context.Background(), r, "/wt/feature-login")
	if !status.Unknown {
		t.Error("expected Unknown=true when IsDirty returns error")
	}
}

func TestCollectWorktreeStatusUnknownOnAheadBehindError(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // pre-cancel so AheadBehind propagates ctx.Err()

	r := &mockRunner{
		run: func(_ ...string) (string, error) { return "", nil },
		// IsDirty (status --porcelain) returns an error too on cancelled ctx, so
		// returning a non-nil error for all runInDir calls is sufficient here.
		runInDir: func(_ string, _ ...string) (string, error) {
			return "", context.Canceled
		},
	}
	status := CollectWorktreeStatus(ctx, r, "/wt/feature-login")
	if !status.Unknown {
		t.Error("expected Unknown=true when git queries return context error")
	}
}

func TestFilterDetailsByStatusDirty(t *testing.T) {
	rows := []resolver.WorktreeDetail{
		{Task: taskLogin, Status: resolver.WorktreeStatus{Dirty: true}},
		{Task: taskSignup, Status: resolver.WorktreeStatus{Dirty: false}},
		{Task: taskLogout, Status: resolver.WorktreeStatus{Dirty: true, Behind: 1}},
	}

	filtered := FilterDetailsByStatus(rows, true, false)
	if len(filtered) != 2 {
		t.Fatalf("expected 2 dirty rows, got %d", len(filtered))
	}
	if filtered[0].Task != taskLogin || filtered[1].Task != taskLogout {
		t.Errorf("unexpected filtered tasks: %v, %v", filtered[0].Task, filtered[1].Task)
	}
}

func TestFilterDetailsByStatusBehind(t *testing.T) {
	rows := []resolver.WorktreeDetail{
		{Task: taskLogin, Status: resolver.WorktreeStatus{Behind: 3}},
		{Task: taskSignup, Status: resolver.WorktreeStatus{Behind: 0}},
	}

	filtered := FilterDetailsByStatus(rows, false, true)
	if len(filtered) != 1 {
		t.Fatalf("expected 1 behind row, got %d", len(filtered))
	}
	if filtered[0].Task != taskLogin {
		t.Errorf("expected 'login', got %q", filtered[0].Task)
	}
}

func TestFilterDetailsByStatusBoth(t *testing.T) {
	rows := []resolver.WorktreeDetail{
		{Task: taskLogin, Status: resolver.WorktreeStatus{Dirty: true, Behind: 3}},
		{Task: taskSignup, Status: resolver.WorktreeStatus{Dirty: true, Behind: 0}},
		{Task: taskLogout, Status: resolver.WorktreeStatus{Dirty: false, Behind: 2}},
	}

	// Both filters active: only rows that are both dirty AND behind
	filtered := FilterDetailsByStatus(rows, true, true)
	if len(filtered) != 1 {
		t.Fatalf("expected 1 row matching both filters, got %d", len(filtered))
	}
	if filtered[0].Task != taskLogin {
		t.Errorf("expected 'login', got %q", filtered[0].Task)
	}
}

func TestFilterDetailsByStatusNoFilter(t *testing.T) {
	rows := []resolver.WorktreeDetail{
		{Task: taskLogin},
		{Task: taskSignup},
	}

	filtered := FilterDetailsByStatus(rows, false, false)
	if len(filtered) != 2 {
		t.Errorf("expected all 2 rows when no filters active, got %d", len(filtered))
	}
}

func TestFilterDetailsByStatusUnknownPassthrough(t *testing.T) {
	rows := []resolver.WorktreeDetail{
		{Task: taskLogin, Status: resolver.WorktreeStatus{Dirty: true}},
		{Task: taskSignup, Status: resolver.WorktreeStatus{Unknown: true}},
		{Task: taskLogout, Status: resolver.WorktreeStatus{Dirty: false}},
	}

	// Under --dirty filter: clean row excluded, but Unknown row must be kept.
	filtered := FilterDetailsByStatus(rows, true, false)
	if len(filtered) != 2 {
		t.Fatalf("expected 2 rows (dirty + unknown), got %d", len(filtered))
	}
	tasks := []string{filtered[0].Task, filtered[1].Task}
	if tasks[0] != taskLogin || tasks[1] != taskSignup {
		t.Errorf("unexpected tasks: %v", tasks)
	}

	// Under --behind filter: non-behind rows excluded, but Unknown row must be kept.
	filtered2 := FilterDetailsByStatus(rows, false, true)
	if len(filtered2) != 1 {
		t.Fatalf("expected 1 row (unknown), got %d", len(filtered2))
	}
	if filtered2[0].Task != taskSignup {
		t.Errorf("expected unknown row 'signup', got %q", filtered2[0].Task)
	}
}

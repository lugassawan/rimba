package operations

import (
	"errors"
	"testing"

	"github.com/lugassawan/rimba/internal/resolver"
)

func TestCollectTasks(t *testing.T) {
	prefixes := resolver.AllPrefixes()

	tests := []struct {
		name      string
		worktrees []resolver.WorktreeInfo
		want      []string
	}{
		{
			name:      "empty",
			worktrees: nil,
			want:      []string{},
		},
		{
			name: "feature and bugfix branches",
			worktrees: []resolver.WorktreeInfo{
				{Branch: branchFeature},
				{Branch: branchBugfixTypo},
				{Branch: branchMain},
			},
			want: []string{"login", "typo", branchMain},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CollectTasks(tt.worktrees, prefixes)
			if len(got) != len(tt.want) {
				t.Fatalf("CollectTasks() = %v (len %d), want %v (len %d)", got, len(got), tt.want, len(tt.want))
			}
			for i, w := range tt.want {
				if got[i] != w {
					t.Errorf("CollectTasks()[%d] = %q, want %q", i, got[i], w)
				}
			}
		})
	}
}

func TestFilterEligibleBasic(t *testing.T) {
	prefixes := resolver.AllPrefixes()
	worktrees := []resolver.WorktreeInfo{
		{Branch: branchMain},
		{Branch: ""},
		{Branch: branchFeature},
		{Branch: branchBugfixTypo},
	}
	allTasks := CollectTasks(worktrees, prefixes)

	eligible := FilterEligible(worktrees, prefixes, branchMain, allTasks, false)
	for _, wt := range eligible {
		if wt.Branch == branchMain || wt.Branch == "" {
			t.Errorf("expected main/empty branch to be filtered, got %q", wt.Branch)
		}
	}
	if len(eligible) != 2 {
		t.Errorf("expected 2 eligible, got %d", len(eligible))
	}
}

func TestFilterEligibleInherited(t *testing.T) {
	prefixes := resolver.AllPrefixes()
	worktrees := []resolver.WorktreeInfo{
		{Branch: branchMain},
		{Branch: branchFeature},
		{Branch: "bugfix/login"},
	}
	allTasks := CollectTasks(worktrees, prefixes)

	eligible := FilterEligible(worktrees, prefixes, branchMain, allTasks, false)
	if len(eligible) > 2 {
		t.Errorf("expected inherited to be filtered, got %d eligible", len(eligible))
	}

	eligibleWithInherited := FilterEligible(worktrees, prefixes, branchMain, allTasks, true)
	if len(eligibleWithInherited) != 2 {
		t.Errorf("expected all 2 non-main to be included, got %d", len(eligibleWithInherited))
	}
}

func TestFilterEligibleNumericSuffix(t *testing.T) {
	prefixes := resolver.AllPrefixes()
	worktrees := []resolver.WorktreeInfo{
		{Branch: branchMain},
		{Branch: "feature/login"},
		{Branch: "feature/login-2"},
	}
	allTasks := CollectTasks(worktrees, prefixes)

	eligible := FilterEligible(worktrees, prefixes, branchMain, allTasks, false)
	if len(eligible) != 1 {
		t.Errorf("expected 1 eligible (login-2 filtered as inherited), got %d", len(eligible))
	}
	if len(eligible) > 0 && eligible[0].Branch != "feature/login" {
		t.Errorf("expected feature/login, got %s", eligible[0].Branch)
	}

	eligibleAll := FilterEligible(worktrees, prefixes, branchMain, allTasks, true)
	if len(eligibleAll) != 2 {
		t.Errorf("expected 2 eligible with includeInherited, got %d", len(eligibleAll))
	}
}

func TestSyncMethodLabel(t *testing.T) {
	if got := SyncMethodLabel(true); got != "Merged" {
		t.Errorf("SyncMethodLabel(true) = %q, want %q", got, "Merged")
	}
	if got := SyncMethodLabel(false); got != "Rebased" {
		t.Errorf("SyncMethodLabel(false) = %q, want %q", got, "Rebased")
	}
}

func TestSyncBranchRebaseSuccess(t *testing.T) {
	r := &mockRunner{
		run:      func(_ ...string) (string, error) { return "", nil },
		runInDir: noopRunInDir,
	}
	if err := SyncBranch(r, "/worktree", branchMain, false); err != nil {
		t.Fatalf("SyncBranch rebase: %v", err)
	}
}

func TestSyncBranchRebaseFailTriggersAbort(t *testing.T) {
	var abortCalled bool
	r := &mockRunner{
		run: func(_ ...string) (string, error) { return "", nil },
		runInDir: func(_ string, args ...string) (string, error) {
			if len(args) >= 2 && args[0] == "rebase" && args[1] == "--abort" {
				abortCalled = true
				return "", nil
			}
			if len(args) >= 1 && args[0] == "rebase" {
				return "", errors.New("conflict")
			}
			return "", nil
		},
	}

	if err := SyncBranch(r, "/worktree", branchMain, false); err == nil {
		t.Fatal("expected rebase error")
	}
	if !abortCalled {
		t.Error("expected rebase --abort to be called after failure")
	}
}

func TestSyncBranchMergeSuccess(t *testing.T) {
	r := &mockRunner{
		run:      func(_ ...string) (string, error) { return "", nil },
		runInDir: noopRunInDir,
	}
	if err := SyncBranch(r, "/worktree", branchMain, true); err != nil {
		t.Fatalf("SyncBranch merge: %v", err)
	}
}

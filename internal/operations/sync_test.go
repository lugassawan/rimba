package operations

import (
	"errors"
	"strings"
	"testing"

	"github.com/lugassawan/rimba/internal/resolver"
)

const (
	cmdRebase       = "rebase"
	fakeUpstreamRef = "origin/feature/login"
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
			if len(args) >= 2 && args[0] == cmdRebase && args[1] == "--abort" {
				abortCalled = true
				return "", nil
			}
			if len(args) >= 1 && args[0] == cmdRebase {
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

func TestSyncWorktreeClean(t *testing.T) {
	r := &mockRunner{
		run:      func(_ ...string) (string, error) { return "", nil },
		runInDir: noopRunInDir,
	}
	wt := resolver.WorktreeInfo{Branch: branchFeature, Path: pathWtFeatureLogin}
	res := SyncWorktree(r, branchMain, wt, false, false)

	if !res.Synced {
		t.Error("expected Synced=true")
	}
	if res.Skipped || res.Failed {
		t.Errorf("unexpected Skipped=%v Failed=%v", res.Skipped, res.Failed)
	}
	if res.Branch != branchFeature {
		t.Errorf("Branch = %q, want %q", res.Branch, branchFeature)
	}
}

func TestSyncWorktreeDirty(t *testing.T) {
	r := &mockRunner{
		run: func(_ ...string) (string, error) { return "", nil },
		runInDir: func(_ string, args ...string) (string, error) {
			if len(args) >= 1 && args[0] == "status" {
				return "M file.go", nil
			}
			return "", nil
		},
	}
	wt := resolver.WorktreeInfo{Branch: branchFeature, Path: pathWtFeatureLogin}
	res := SyncWorktree(r, branchMain, wt, false, false)

	if !res.Skipped {
		t.Error("expected Skipped=true for dirty worktree")
	}
	if res.SkipReason != "dirty" {
		t.Errorf("SkipReason = %q, want %q", res.SkipReason, "dirty")
	}
}

func TestSyncWorktreeIsDirtyError(t *testing.T) {
	r := &mockRunner{
		run: func(_ ...string) (string, error) { return "", nil },
		runInDir: func(_ string, _ ...string) (string, error) {
			return "", errGitFailed
		},
	}
	wt := resolver.WorktreeInfo{Branch: branchFeature, Path: pathWtFeatureLogin}
	res := SyncWorktree(r, branchMain, wt, false, false)

	if !res.Skipped {
		t.Error("expected Skipped=true for IsDirty error")
	}
	if !strings.Contains(res.SkipReason, "could not check status") {
		t.Errorf("SkipReason = %q, want 'could not check status'", res.SkipReason)
	}
}

func TestSyncWorktreeSyncFailure(t *testing.T) {
	r := &mockRunner{
		run: func(_ ...string) (string, error) { return "", nil },
		runInDir: func(_ string, args ...string) (string, error) {
			if len(args) >= 1 && args[0] == cmdRebase {
				return "", errors.New("conflict")
			}
			return "", nil
		},
	}
	wt := resolver.WorktreeInfo{Branch: branchFeature, Path: pathWtFeatureLogin}
	res := SyncWorktree(r, branchMain, wt, false, false)

	if !res.Failed {
		t.Error("expected Failed=true")
	}
	if !strings.Contains(res.FailureHint, "rebase") {
		t.Errorf("FailureHint = %q, want 'rebase'", res.FailureHint)
	}
}

func TestSyncWorktreeMergeFailure(t *testing.T) {
	r := &mockRunner{
		run: func(_ ...string) (string, error) { return "", nil },
		runInDir: func(_ string, args ...string) (string, error) {
			if len(args) >= 1 && args[0] == "merge" {
				return "", errors.New("merge conflict")
			}
			return "", nil
		},
	}
	wt := resolver.WorktreeInfo{Branch: branchFeature, Path: pathWtFeatureLogin}
	res := SyncWorktree(r, branchMain, wt, true, false)

	if !res.Failed {
		t.Error("expected Failed=true")
	}
	if !strings.Contains(res.FailureHint, "merge") {
		t.Errorf("FailureHint = %q, want 'merge'", res.FailureHint)
	}
}

func TestPushBranchRebase(t *testing.T) {
	var capturedArgs []string
	r := &mockRunner{
		run: func(_ ...string) (string, error) { return "", nil },
		runInDir: func(_ string, args ...string) (string, error) {
			if len(args) >= 1 && args[0] == cmdRevParse {
				return fakeUpstreamRef, nil // has upstream
			}
			capturedArgs = args
			return "", nil
		},
	}

	pushed, skipped, err := PushBranch(r, pathWtFeatureLogin, false)
	if err != nil {
		t.Fatalf("PushBranch: %v", err)
	}
	if !pushed || skipped {
		t.Errorf("pushed=%v skipped=%v, want pushed=true skipped=false", pushed, skipped)
	}
	if len(capturedArgs) < 2 || capturedArgs[1] != "--force-with-lease" {
		t.Errorf("expected push --force-with-lease, got %v", capturedArgs)
	}
}

func TestPushBranchMerge(t *testing.T) {
	var capturedArgs []string
	r := &mockRunner{
		run: func(_ ...string) (string, error) { return "", nil },
		runInDir: func(_ string, args ...string) (string, error) {
			if len(args) >= 1 && args[0] == cmdRevParse {
				return fakeUpstreamRef, nil
			}
			capturedArgs = args
			return "", nil
		},
	}

	pushed, skipped, err := PushBranch(r, pathWtFeatureLogin, true)
	if err != nil {
		t.Fatalf("PushBranch: %v", err)
	}
	if !pushed || skipped {
		t.Errorf("pushed=%v skipped=%v, want pushed=true skipped=false", pushed, skipped)
	}
	if len(capturedArgs) != 1 || capturedArgs[0] != "push" {
		t.Errorf("expected [push], got %v", capturedArgs)
	}
}

func TestPushBranchNoUpstream(t *testing.T) {
	r := &mockRunner{
		run: func(_ ...string) (string, error) { return "", nil },
		runInDir: func(_ string, _ ...string) (string, error) {
			return "", errors.New("no upstream")
		},
	}

	pushed, skipped, err := PushBranch(r, pathWtFeatureLogin, false)
	if err != nil {
		t.Fatalf("PushBranch: %v", err)
	}
	if pushed || !skipped {
		t.Errorf("pushed=%v skipped=%v, want pushed=false skipped=true", pushed, skipped)
	}
}

func TestPushBranchError(t *testing.T) {
	r := &mockRunner{
		run: func(_ ...string) (string, error) { return "", nil },
		runInDir: func(_ string, args ...string) (string, error) {
			if len(args) >= 1 && args[0] == cmdRevParse {
				return fakeUpstreamRef, nil
			}
			return "", errors.New("push rejected")
		},
	}

	pushed, _, err := PushBranch(r, pathWtFeatureLogin, false)
	if err == nil {
		t.Fatal("expected error from PushBranch")
	}
	if pushed {
		t.Error("expected pushed=false on error")
	}
	if !strings.Contains(err.Error(), "push rejected") {
		t.Errorf("error = %q, want 'push rejected'", err.Error())
	}
}

func TestSyncWorktreePushSuccess(t *testing.T) {
	r := &mockRunner{
		run: func(_ ...string) (string, error) { return "", nil },
		runInDir: func(_ string, args ...string) (string, error) {
			if len(args) >= 1 && args[0] == cmdRevParse {
				return fakeUpstreamRef, nil
			}
			return "", nil
		},
	}
	wt := resolver.WorktreeInfo{Branch: branchFeature, Path: pathWtFeatureLogin}
	res := SyncWorktree(r, branchMain, wt, false, true)

	if !res.Synced {
		t.Error("expected Synced=true")
	}
	if !res.Pushed {
		t.Error("expected Pushed=true")
	}
	if res.PushSkipped || res.PushFailed {
		t.Errorf("PushSkipped=%v PushFailed=%v, want both false", res.PushSkipped, res.PushFailed)
	}
}

func TestSyncWorktreePushSkipped(t *testing.T) {
	r := &mockRunner{
		run: func(_ ...string) (string, error) { return "", nil },
		runInDir: func(_ string, args ...string) (string, error) {
			if len(args) >= 1 && args[0] == cmdRevParse {
				return "", errors.New("no upstream")
			}
			return "", nil
		},
	}
	wt := resolver.WorktreeInfo{Branch: branchFeature, Path: pathWtFeatureLogin}
	res := SyncWorktree(r, branchMain, wt, false, true)

	if !res.Synced {
		t.Error("expected Synced=true")
	}
	if !res.PushSkipped {
		t.Error("expected PushSkipped=true")
	}
	if res.Pushed || res.PushFailed {
		t.Errorf("Pushed=%v PushFailed=%v, want both false", res.Pushed, res.PushFailed)
	}
}

func TestSyncWorktreePushFailed(t *testing.T) {
	r := &mockRunner{
		run: func(_ ...string) (string, error) { return "", nil },
		runInDir: func(_ string, args ...string) (string, error) {
			if len(args) >= 1 && args[0] == cmdRevParse {
				return fakeUpstreamRef, nil
			}
			if len(args) >= 1 && args[0] == "push" {
				return "", errors.New("push rejected")
			}
			return "", nil
		},
	}
	wt := resolver.WorktreeInfo{Branch: branchFeature, Path: pathWtFeatureLogin}
	res := SyncWorktree(r, branchMain, wt, false, true)

	if !res.Synced {
		t.Error("expected Synced=true")
	}
	if !res.PushFailed {
		t.Error("expected PushFailed=true")
	}
	if res.PushError == "" {
		t.Error("expected PushError to be set")
	}
}

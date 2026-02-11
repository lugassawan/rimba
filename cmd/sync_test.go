package cmd

import (
	"errors"
	"strings"
	"testing"

	"github.com/lugassawan/rimba/internal/resolver"
)

const cmdRebase = "rebase"

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
				{Branch: "bugfix/typo"},
				{Branch: branchMain},
			},
			want: []string{"login", "typo", branchMain},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := collectTasks(tt.worktrees, prefixes)
			if len(got) != len(tt.want) {
				t.Fatalf("collectTasks() = %v (len %d), want %v (len %d)", got, len(got), tt.want, len(tt.want))
			}
			for i, w := range tt.want {
				if got[i] != w {
					t.Errorf("collectTasks()[%d] = %q, want %q", i, got[i], w)
				}
			}
		})
	}
}

func TestFilterEligible(t *testing.T) {
	prefixes := resolver.AllPrefixes()

	worktrees := []resolver.WorktreeInfo{
		{Branch: branchMain},
		{Branch: ""},
		{Branch: branchFeature},
		{Branch: "bugfix/typo"},
	}
	allTasks := collectTasks(worktrees, prefixes)

	t.Run("filters main and empty", func(t *testing.T) {
		eligible := filterEligible(worktrees, prefixes, branchMain, allTasks, false)
		for _, wt := range eligible {
			if wt.Branch == branchMain || wt.Branch == "" {
				t.Errorf("expected main/empty branch to be filtered, got %q", wt.Branch)
			}
		}
		if len(eligible) != 2 {
			t.Errorf("expected 2 eligible, got %d", len(eligible))
		}
	})

	t.Run("inherited filtering", func(t *testing.T) {
		worktreesWithInherited := []resolver.WorktreeInfo{
			{Branch: branchMain},
			{Branch: branchFeature},
			{Branch: "bugfix/login"},
		}
		allTasksInherited := collectTasks(worktreesWithInherited, prefixes)

		eligible := filterEligible(worktreesWithInherited, prefixes, branchMain, allTasksInherited, false)
		if len(eligible) > 2 {
			t.Errorf("expected inherited to be filtered, got %d eligible", len(eligible))
		}

		eligibleWithInherited := filterEligible(worktreesWithInherited, prefixes, branchMain, allTasksInherited, true)
		if len(eligibleWithInherited) != 2 {
			t.Errorf("expected all 2 non-main to be included, got %d", len(eligibleWithInherited))
		}
	})
}

func TestSyncMethodLabel(t *testing.T) {
	if got := syncMethodLabel(true); got != "Merged" {
		t.Errorf("syncMethodLabel(true) = %q, want %q", got, "Merged")
	}
	if got := syncMethodLabel(false); got != "Rebased" {
		t.Errorf("syncMethodLabel(false) = %q, want %q", got, "Rebased")
	}
}

func TestDoSyncRebaseSuccess(t *testing.T) {
	r := &mockRunner{
		run:      func(_ ...string) (string, error) { return "", nil },
		runInDir: noopRunInDir,
	}
	if err := doSync(r, pathWorktree, branchMain, false); err != nil {
		t.Fatalf("doSync rebase: %v", err)
	}
}

func TestDoSyncRebaseFailTriggersAbort(t *testing.T) {
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

	if err := doSync(r, pathWorktree, branchMain, false); err == nil {
		t.Fatal("expected rebase error")
	}
	if !abortCalled {
		t.Error("expected rebase --abort to be called after failure")
	}
}

func TestDoSyncMergeSuccess(t *testing.T) {
	r := &mockRunner{
		run:      func(_ ...string) (string, error) { return "", nil },
		runInDir: noopRunInDir,
	}
	if err := doSync(r, pathWorktree, branchMain, true); err != nil {
		t.Fatalf("doSync merge: %v", err)
	}
}

func TestPrintSyncSummary(t *testing.T) {
	t.Run("all synced", func(t *testing.T) {
		cmd, buf := newTestCmd()
		res := &syncResult{synced: 3}
		printSyncSummary(cmd, branchMain, false, res)
		if !strings.Contains(buf.String(), "Rebased 3 worktree(s)") {
			t.Errorf("output = %q, want 'Rebased 3 worktree(s)'", buf.String())
		}
	})

	t.Run("with dirty skips", func(t *testing.T) {
		cmd, buf := newTestCmd()
		res := &syncResult{synced: 2, skippedDirty: 1}
		printSyncSummary(cmd, branchMain, false, res)
		if !strings.Contains(buf.String(), "1 skipped (dirty)") {
			t.Errorf("output = %q, want dirty skip info", buf.String())
		}
	})

	t.Run("with failures", func(t *testing.T) {
		cmd, buf := newTestCmd()
		res := &syncResult{synced: 1, failed: 1, failures: []string{"  feature/x: To resolve: cd /wt && git rebase main"}}
		printSyncSummary(cmd, branchMain, false, res)
		out := buf.String()
		if !strings.Contains(out, "1 failed (conflict)") {
			t.Errorf("output = %q, want failure info", out)
		}
		if !strings.Contains(out, "feature/x") {
			t.Errorf("output = %q, want failure details", out)
		}
	})

	t.Run("merge label", func(t *testing.T) {
		cmd, buf := newTestCmd()
		res := &syncResult{synced: 1}
		printSyncSummary(cmd, branchMain, true, res)
		if !strings.Contains(buf.String(), "Merged") {
			t.Errorf("output = %q, want 'Merged'", buf.String())
		}
	})
}

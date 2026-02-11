package cmd

import (
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/lugassawan/rimba/internal/config"
	"github.com/lugassawan/rimba/internal/resolver"
	"github.com/lugassawan/rimba/internal/spinner"
	"github.com/spf13/cobra"
)

const cmdRebase = "rebase"

func testSyncConfig() *config.Config {
	return &config.Config{DefaultSource: branchMain}
}

func testSyncPrefixes() []string {
	return resolver.AllPrefixes()
}

func testSyncSpinner(cmd *cobra.Command) *spinner.Spinner {
	return spinner.New(spinnerOpts(cmd))
}

func testSyncWorktrees() []resolver.WorktreeInfo {
	return []resolver.WorktreeInfo{
		{Branch: branchMain, Path: "/repo"},
		{Branch: branchFeature, Path: pathWtFeatureLogin},
		{Branch: branchBugfixTypo, Path: "/wt/bugfix-typo"},
	}
}

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

func TestFilterEligibleBasic(t *testing.T) {
	prefixes := resolver.AllPrefixes()
	worktrees := []resolver.WorktreeInfo{
		{Branch: branchMain},
		{Branch: ""},
		{Branch: branchFeature},
		{Branch: branchBugfixTypo},
	}
	allTasks := collectTasks(worktrees, prefixes)

	eligible := filterEligible(worktrees, prefixes, branchMain, allTasks, false)
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
	allTasks := collectTasks(worktrees, prefixes)

	eligible := filterEligible(worktrees, prefixes, branchMain, allTasks, false)
	if len(eligible) > 2 {
		t.Errorf("expected inherited to be filtered, got %d eligible", len(eligible))
	}

	eligibleWithInherited := filterEligible(worktrees, prefixes, branchMain, allTasks, true)
	if len(eligibleWithInherited) != 2 {
		t.Errorf("expected all 2 non-main to be included, got %d", len(eligibleWithInherited))
	}
}

func TestFilterEligibleNumericSuffix(t *testing.T) {
	prefixes := resolver.AllPrefixes()
	// login-2 is inherited from login (numeric suffix)
	worktrees := []resolver.WorktreeInfo{
		{Branch: branchMain},
		{Branch: "feature/login"},
		{Branch: "feature/login-2"},
	}
	allTasks := collectTasks(worktrees, prefixes)

	eligible := filterEligible(worktrees, prefixes, branchMain, allTasks, false)
	if len(eligible) != 1 {
		t.Errorf("expected 1 eligible (login-2 filtered as inherited), got %d", len(eligible))
	}
	if len(eligible) > 0 && eligible[0].Branch != "feature/login" {
		t.Errorf("expected feature/login, got %s", eligible[0].Branch)
	}

	// With includeInherited=true, both should be included
	eligibleAll := filterEligible(worktrees, prefixes, branchMain, allTasks, true)
	if len(eligibleAll) != 2 {
		t.Errorf("expected 2 eligible with includeInherited, got %d", len(eligibleAll))
	}
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

func TestSyncOneSuccess(t *testing.T) {
	worktrees := testSyncWorktrees()

	t.Run("rebase", func(t *testing.T) {
		cmd, buf := newTestCmd()
		r := &mockRunner{
			run:      func(_ ...string) (string, error) { return "", nil },
			runInDir: noopRunInDir,
		}
		sc := syncContext{cmd: cmd, r: r, cfg: testSyncConfig(), s: testSyncSpinner(cmd)}

		err := syncOne(sc, "login", worktrees, testSyncPrefixes(), false)
		if err != nil {
			t.Fatalf("syncOne: %v", err)
		}
		if !strings.Contains(buf.String(), "Rebased") {
			t.Errorf("output = %q, want 'Rebased'", buf.String())
		}
	})

	t.Run("merge", func(t *testing.T) {
		cmd, buf := newTestCmd()
		r := &mockRunner{
			run:      func(_ ...string) (string, error) { return "", nil },
			runInDir: noopRunInDir,
		}
		sc := syncContext{cmd: cmd, r: r, cfg: testSyncConfig(), s: testSyncSpinner(cmd)}

		err := syncOne(sc, "login", worktrees, testSyncPrefixes(), true)
		if err != nil {
			t.Fatalf("syncOne: %v", err)
		}
		if !strings.Contains(buf.String(), "Merged") {
			t.Errorf("output = %q, want 'Merged'", buf.String())
		}
	})
}

func TestSyncOneNotFound(t *testing.T) {
	worktrees := testSyncWorktrees()
	cmd, _ := newTestCmd()
	r := &mockRunner{
		run:      func(_ ...string) (string, error) { return "", nil },
		runInDir: noopRunInDir,
	}
	sc := syncContext{cmd: cmd, r: r, cfg: testSyncConfig(), s: testSyncSpinner(cmd)}

	err := syncOne(sc, "nonexistent", worktrees, testSyncPrefixes(), false)
	if err == nil {
		t.Fatal("expected error for unknown task")
	}
}

func TestSyncOneDirty(t *testing.T) {
	worktrees := testSyncWorktrees()
	cmd, _ := newTestCmd()
	r := &mockRunner{
		run: func(_ ...string) (string, error) { return "", nil },
		runInDir: func(_ string, args ...string) (string, error) {
			if len(args) >= 1 && args[0] == cmdStatus {
				return dirtyOutput, nil
			}
			return "", nil
		},
	}
	sc := syncContext{cmd: cmd, r: r, cfg: testSyncConfig(), s: testSyncSpinner(cmd)}

	err := syncOne(sc, "login", worktrees, testSyncPrefixes(), false)
	if err == nil {
		t.Fatal("expected error for dirty worktree")
	}
	if !strings.Contains(err.Error(), "uncommitted changes") {
		t.Errorf("error = %q, want 'uncommitted changes'", err.Error())
	}
}

func TestSyncOneErrorPaths(t *testing.T) {
	worktrees := testSyncWorktrees()

	t.Run("IsDirty error", func(t *testing.T) {
		cmd, _ := newTestCmd()
		r := &mockRunner{
			run: func(_ ...string) (string, error) { return "", nil },
			runInDir: func(_ string, _ ...string) (string, error) {
				return "", errGitFailed
			},
		}
		sc := syncContext{cmd: cmd, r: r, cfg: testSyncConfig(), s: testSyncSpinner(cmd)}

		err := syncOne(sc, "login", worktrees, testSyncPrefixes(), false)
		if err == nil {
			t.Fatal("expected error from IsDirty failure")
		}
	})

	t.Run("doSync failure", func(t *testing.T) {
		cmd, _ := newTestCmd()
		r := &mockRunner{
			run: func(_ ...string) (string, error) { return "", nil },
			runInDir: func(_ string, args ...string) (string, error) {
				if len(args) >= 1 && args[0] == cmdRebase {
					return "", errors.New("conflict")
				}
				return "", nil
			},
		}
		sc := syncContext{cmd: cmd, r: r, cfg: testSyncConfig(), s: testSyncSpinner(cmd)}

		err := syncOne(sc, "login", worktrees, testSyncPrefixes(), false)
		if err == nil {
			t.Fatal("expected error from doSync failure")
		}
	})
}

func TestSyncAllClean(t *testing.T) {
	worktrees := testSyncWorktrees()
	cmd, buf := newTestCmd()
	r := &mockRunner{
		run:      func(_ ...string) (string, error) { return "", nil },
		runInDir: noopRunInDir,
	}
	sc := syncContext{cmd: cmd, r: r, cfg: testSyncConfig(), s: testSyncSpinner(cmd)}

	err := syncAll(sc, worktrees, testSyncPrefixes(), false, false)
	if err != nil {
		t.Fatalf(fatalSyncAll, err)
	}
	if !strings.Contains(buf.String(), "Rebased 2 worktree(s)") {
		t.Errorf("output = %q, want 'Rebased 2 worktree(s)'", buf.String())
	}
}

func TestSyncAllDirtySkip(t *testing.T) {
	worktrees := testSyncWorktrees()
	cmd, buf := newTestCmd()
	r := &mockRunner{
		run: func(_ ...string) (string, error) { return "", nil },
		runInDir: func(dir string, args ...string) (string, error) {
			if len(args) >= 1 && args[0] == cmdStatus {
				if strings.Contains(dir, "login") {
					return dirtyOutput, nil
				}
			}
			return "", nil
		},
	}
	sc := syncContext{cmd: cmd, r: r, cfg: testSyncConfig(), s: testSyncSpinner(cmd)}

	err := syncAll(sc, worktrees, testSyncPrefixes(), false, false)
	if err != nil {
		t.Fatalf(fatalSyncAll, err)
	}
	out := buf.String()
	if !strings.Contains(out, "1 skipped (dirty)") {
		t.Errorf("output = %q, want dirty skip count", out)
	}
}

func TestSyncAllInherited(t *testing.T) {
	worktrees := testSyncWorktrees()
	cmd, _ := newTestCmd()
	r := &mockRunner{
		run:      func(_ ...string) (string, error) { return "", nil },
		runInDir: noopRunInDir,
	}
	sc := syncContext{cmd: cmd, r: r, cfg: testSyncConfig(), s: testSyncSpinner(cmd)}

	err := syncAll(sc, worktrees, testSyncPrefixes(), false, true)
	if err != nil {
		t.Fatalf(fatalSyncAll, err)
	}
}

func TestSyncWorktreeClean(t *testing.T) {
	cmd, _ := newTestCmd()
	r := &mockRunner{
		run:      func(_ ...string) (string, error) { return "", nil },
		runInDir: noopRunInDir,
	}

	var res syncResult
	var mu sync.Mutex
	wt := resolver.WorktreeInfo{Branch: branchFeature, Path: pathWtFeatureLogin}
	syncWorktree(cmd, r, branchMain, wt, false, &res, &mu)

	if res.synced != 1 {
		t.Errorf("synced = %d, want 1", res.synced)
	}
}

func TestSyncWorktreeDirtyAndError(t *testing.T) {
	t.Run("dirty worktree skipped", func(t *testing.T) {
		cmd, buf := newTestCmd()
		r := &mockRunner{
			run: func(_ ...string) (string, error) { return "", nil },
			runInDir: func(_ string, args ...string) (string, error) {
				if len(args) >= 1 && args[0] == cmdStatus {
					return "M file.go", nil
				}
				return "", nil
			},
		}

		var res syncResult
		var mu sync.Mutex
		wt := resolver.WorktreeInfo{Branch: branchFeature, Path: pathWtFeatureLogin}
		syncWorktree(cmd, r, branchMain, wt, false, &res, &mu)

		if res.skippedDirty != 1 {
			t.Errorf("skippedDirty = %d, want 1", res.skippedDirty)
		}
		if !strings.Contains(buf.String(), "Skipping") {
			t.Errorf("output = %q, want 'Skipping'", buf.String())
		}
	})

	t.Run("IsDirty error skipped", func(t *testing.T) {
		cmd, buf := newTestCmd()
		r := &mockRunner{
			run: func(_ ...string) (string, error) { return "", nil },
			runInDir: func(_ string, _ ...string) (string, error) {
				return "", errGitFailed
			},
		}

		var res syncResult
		var mu sync.Mutex
		wt := resolver.WorktreeInfo{Branch: branchFeature, Path: pathWtFeatureLogin}
		syncWorktree(cmd, r, branchMain, wt, false, &res, &mu)

		if res.skippedDirty != 1 {
			t.Errorf("skippedDirty = %d, want 1", res.skippedDirty)
		}
		if !strings.Contains(buf.String(), "Warning") {
			t.Errorf("output = %q, want warning", buf.String())
		}
	})
}

func TestSyncWorktreeDoSyncFailure(t *testing.T) {
	cmd, _ := newTestCmd()
	r := &mockRunner{
		run: func(_ ...string) (string, error) { return "", nil },
		runInDir: func(_ string, args ...string) (string, error) {
			if len(args) >= 1 && args[0] == cmdRebase {
				return "", errors.New("conflict")
			}
			return "", nil
		},
	}

	var res syncResult
	var mu sync.Mutex
	wt := resolver.WorktreeInfo{Branch: branchFeature, Path: pathWtFeatureLogin}
	syncWorktree(cmd, r, branchMain, wt, false, &res, &mu)

	if res.failed != 1 {
		t.Errorf("failed = %d, want 1", res.failed)
	}
	if len(res.failures) != 1 {
		t.Errorf("failures = %d, want 1", len(res.failures))
	}
}

func TestSyncWorktreeMergeFailure(t *testing.T) {
	cmd, _ := newTestCmd()
	r := &mockRunner{
		run: func(_ ...string) (string, error) { return "", nil },
		runInDir: func(_ string, args ...string) (string, error) {
			if len(args) >= 1 && args[0] == flagSyncMerge {
				return "", errors.New("merge conflict")
			}
			return "", nil
		},
	}

	var res syncResult
	var mu sync.Mutex
	wt := resolver.WorktreeInfo{Branch: branchFeature, Path: pathWtFeatureLogin}
	syncWorktree(cmd, r, branchMain, wt, true, &res, &mu)

	if res.failed != 1 {
		t.Errorf("failed = %d, want 1", res.failed)
	}
	if len(res.failures) > 0 && !strings.Contains(res.failures[0], "merge") {
		t.Errorf("failure message = %q, want 'merge'", res.failures[0])
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

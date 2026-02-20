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

const (
	cmdRebase       = "rebase"
	fakeUpstreamRef = "origin/feature/login"
)

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

func TestSyncOneSuccess(t *testing.T) {
	worktrees := testSyncWorktrees()

	t.Run("rebase", func(t *testing.T) {
		cmd, buf := newTestCmd()
		r := &mockRunner{
			run:      func(_ ...string) (string, error) { return "", nil },
			runInDir: noopRunInDir,
		}
		sc := syncContext{cmd: cmd, r: r, cfg: testSyncConfig(), s: testSyncSpinner(cmd)}

		err := syncOne(sc, "login", worktrees, testSyncPrefixes(), false, false)
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

		err := syncOne(sc, "login", worktrees, testSyncPrefixes(), true, false)
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

	err := syncOne(sc, "nonexistent", worktrees, testSyncPrefixes(), false, false)
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

	err := syncOne(sc, "login", worktrees, testSyncPrefixes(), false, false)
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

		err := syncOne(sc, "login", worktrees, testSyncPrefixes(), false, false)
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

		err := syncOne(sc, "login", worktrees, testSyncPrefixes(), false, false)
		if err == nil {
			t.Fatal("expected error from doSync failure")
		}
	})
}

func TestSyncOnePushSuccess(t *testing.T) {
	worktrees := testSyncWorktrees()
	cmd, buf := newTestCmd()
	r := &mockRunner{
		run: func(_ ...string) (string, error) { return "", nil },
		runInDir: func(_ string, args ...string) (string, error) {
			if len(args) >= 1 && args[0] == cmdRevParse {
				return fakeUpstreamRef, nil
			}
			return "", nil
		},
	}
	sc := syncContext{cmd: cmd, r: r, cfg: testSyncConfig(), s: testSyncSpinner(cmd)}

	err := syncOne(sc, "login", worktrees, testSyncPrefixes(), false, true)
	if err != nil {
		t.Fatalf("syncOne: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "Pushed") {
		t.Errorf("output = %q, want 'Pushed'", out)
	}
}

func TestSyncOnePushSkippedNoUpstream(t *testing.T) {
	worktrees := testSyncWorktrees()
	cmd, buf := newTestCmd()
	r := &mockRunner{
		run: func(_ ...string) (string, error) { return "", nil },
		runInDir: func(_ string, args ...string) (string, error) {
			if len(args) >= 1 && args[0] == cmdRevParse {
				return "", errors.New("no upstream")
			}
			return "", nil
		},
	}
	sc := syncContext{cmd: cmd, r: r, cfg: testSyncConfig(), s: testSyncSpinner(cmd)}

	err := syncOne(sc, "login", worktrees, testSyncPrefixes(), false, true)
	if err != nil {
		t.Fatalf("syncOne: %v", err)
	}
	out := buf.String()
	if strings.Contains(out, "Pushed") {
		t.Errorf("output = %q, should not contain 'Pushed' when no upstream", out)
	}
}

func TestSyncOnePushFailure(t *testing.T) {
	worktrees := testSyncWorktrees()
	cmd, _ := newTestCmd()
	r := &mockRunner{
		run: func(_ ...string) (string, error) { return "", nil },
		runInDir: func(_ string, args ...string) (string, error) {
			if len(args) >= 1 && args[0] == cmdRevParse {
				return fakeUpstreamRef, nil
			}
			if len(args) >= 1 && args[0] == "push" {
				return "", errors.New("rejected")
			}
			return "", nil
		},
	}
	sc := syncContext{cmd: cmd, r: r, cfg: testSyncConfig(), s: testSyncSpinner(cmd)}

	err := syncOne(sc, "login", worktrees, testSyncPrefixes(), false, true)
	if err == nil {
		t.Fatal("expected error from push failure")
	}
	if !strings.Contains(err.Error(), "push failed") {
		t.Errorf("error = %q, want 'push failed'", err.Error())
	}
	if !strings.Contains(err.Error(), "force-with-lease") {
		t.Errorf("error = %q, want recovery hint with force-with-lease", err.Error())
	}
}

func TestSyncAllClean(t *testing.T) {
	worktrees := testSyncWorktrees()
	cmd, buf := newTestCmd()
	r := &mockRunner{
		run:      func(_ ...string) (string, error) { return "", nil },
		runInDir: noopRunInDir,
	}
	sc := syncContext{cmd: cmd, r: r, cfg: testSyncConfig(), s: testSyncSpinner(cmd)}

	err := syncAll(sc, worktrees, testSyncPrefixes(), false, false, false)
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

	err := syncAll(sc, worktrees, testSyncPrefixes(), false, false, false)
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

	err := syncAll(sc, worktrees, testSyncPrefixes(), false, true, false)
	if err != nil {
		t.Fatalf(fatalSyncAll, err)
	}
}

func TestSyncAllPushDefault(t *testing.T) {
	worktrees := testSyncWorktrees()
	cmd, buf := newTestCmd()
	r := &mockRunner{
		run: func(_ ...string) (string, error) { return "", nil },
		runInDir: func(_ string, args ...string) (string, error) {
			if len(args) >= 1 && args[0] == cmdRevParse {
				return fakeUpstreamRef, nil
			}
			return "", nil
		},
	}
	sc := syncContext{cmd: cmd, r: r, cfg: testSyncConfig(), s: testSyncSpinner(cmd)}

	err := syncAll(sc, worktrees, testSyncPrefixes(), false, false, true)
	if err != nil {
		t.Fatalf(fatalSyncAll, err)
	}
	out := buf.String()
	if !strings.Contains(out, "2 pushed") {
		t.Errorf("output = %q, want '2 pushed'", out)
	}
}

func TestSyncAllPushFailure(t *testing.T) {
	worktrees := testSyncWorktrees()
	cmd, buf := newTestCmd()
	r := &mockRunner{
		run: func(_ ...string) (string, error) { return "", nil },
		runInDir: func(_ string, args ...string) (string, error) {
			if len(args) >= 1 && args[0] == cmdRevParse {
				return fakeUpstreamRef, nil
			}
			if len(args) >= 1 && args[0] == "push" {
				return "", errors.New("rejected")
			}
			return "", nil
		},
	}
	sc := syncContext{cmd: cmd, r: r, cfg: testSyncConfig(), s: testSyncSpinner(cmd)}

	err := syncAll(sc, worktrees, testSyncPrefixes(), false, false, true)
	if err != nil {
		t.Fatalf(fatalSyncAll, err)
	}
	out := buf.String()
	if !strings.Contains(out, "push failed") {
		t.Errorf("output = %q, want 'push failed'", out)
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
	syncWorktree(cmd, r, branchMain, wt, false, false, &res, &mu)

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
		syncWorktree(cmd, r, branchMain, wt, false, false, &res, &mu)

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
		syncWorktree(cmd, r, branchMain, wt, false, false, &res, &mu)

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
	syncWorktree(cmd, r, branchMain, wt, false, false, &res, &mu)

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
	syncWorktree(cmd, r, branchMain, wt, true, false, &res, &mu)

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

	t.Run("with pushed count", func(t *testing.T) {
		cmd, buf := newTestCmd()
		res := &syncResult{synced: 3, pushed: 2}
		printSyncSummary(cmd, branchMain, false, res)
		out := buf.String()
		if !strings.Contains(out, "2 pushed") {
			t.Errorf("output = %q, want '2 pushed'", out)
		}
	})

	t.Run("with push failures", func(t *testing.T) {
		cmd, buf := newTestCmd()
		res := &syncResult{synced: 2, pushed: 1, pushFailed: 1, failures: []string{"  feature/x: push failed: rejected"}}
		printSyncSummary(cmd, branchMain, false, res)
		out := buf.String()
		if !strings.Contains(out, "1 push failed") {
			t.Errorf("output = %q, want '1 push failed'", out)
		}
		if !strings.Contains(out, "1 pushed") {
			t.Errorf("output = %q, want '1 pushed'", out)
		}
	})
}

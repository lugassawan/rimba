package cmd

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/lugassawan/rimba/internal/config"
	"github.com/lugassawan/rimba/internal/resolver"
	"github.com/lugassawan/rimba/internal/spinner"
	"github.com/spf13/cobra"
)

const (
	cmdRebase       = "rebase"
	fakeUpstreamRef = "origin/feature/login"
	wantSyncCommand = "sync"
)

func testSyncConfig() *config.Config {
	return &config.Config{DefaultSource: branchMain}
}

func testSyncPrefixes() []string {
	return resolver.DefaultPrefixSet().Strip()
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

func TestPrintSyncDryRunMerge(t *testing.T) {
	cmd, buf := newTestCmd()

	printSyncDryRun(cmd, "feature/login", "main", true, true)

	want := "[dry-run] would merge feature/login onto main\n" +
		"[dry-run] would push feature/login to origin\n"
	if got := buf.String(); got != want {
		t.Fatalf("output = %q, want %q", got, want)
	}
}

func TestPrintSyncDryRunRebaseNoPush(t *testing.T) {
	cmd, buf := newTestCmd()

	printSyncDryRun(cmd, "feature/login", "main", false, false)

	got := buf.String()
	if !strings.Contains(got, "would rebase") {
		t.Errorf("output = %q, want 'would rebase'", got)
	}
	if strings.Contains(got, "would push") {
		t.Errorf("output = %q, should not contain 'would push'", got)
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
		sc := &syncContext{cmd: cmd, r: r, cfg: testSyncConfig(), s: testSyncSpinner(cmd)}

		err := syncOne(context.Background(), sc, "login", worktrees, testSyncPrefixes(), false, false)
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
		sc := &syncContext{cmd: cmd, r: r, cfg: testSyncConfig(), s: testSyncSpinner(cmd)}

		err := syncOne(context.Background(), sc, "login", worktrees, testSyncPrefixes(), true, false)
		if err != nil {
			t.Fatalf("syncOne: %v", err)
		}
		if !strings.Contains(buf.String(), "Merged") {
			t.Errorf("output = %q, want 'Merged'", buf.String())
		}
	})
}

func TestSyncOneOrphanedHardErrors(t *testing.T) {
	// Only "TASK-" is configured, so the "PROJ-*" branch is orphaned;
	// sync has no --force flag, so this guard can never be bypassed here.
	cfg := &config.Config{
		DefaultSource: branchMain,
		Resolver: &config.ResolverConfig{
			Prefix: []config.PrefixEntry{{Prefix: "TASK-"}},
		},
	}
	worktrees := []resolver.WorktreeInfo{
		{Branch: branchMain, Path: "/repo"},
		{Branch: "PROJ-123", Path: "/wt/proj-123"},
	}

	cmd, _ := newTestCmd()
	r := &mockRunner{
		run:      func(_ ...string) (string, error) { return "", nil },
		runInDir: noopRunInDir,
	}
	sc := &syncContext{cmd: cmd, r: r, cfg: cfg, s: testSyncSpinner(cmd)}

	err := syncOne(context.Background(), sc, "PROJ-123", worktrees, cfg.PrefixSet().Strip(), false, false)
	if err == nil {
		t.Fatal("expected orphan-guard error, got nil")
	}
	if !strings.Contains(err.Error(), "re-add the prefix") {
		t.Errorf("error = %q, want it to mention re-adding the prefix", err.Error())
	}
}

func TestSyncOneNotFound(t *testing.T) {
	worktrees := testSyncWorktrees()
	cmd, _ := newTestCmd()
	r := &mockRunner{
		run:      func(_ ...string) (string, error) { return "", nil },
		runInDir: noopRunInDir,
	}
	sc := &syncContext{cmd: cmd, r: r, cfg: testSyncConfig(), s: testSyncSpinner(cmd)}

	err := syncOne(context.Background(), sc, "nonexistent", worktrees, testSyncPrefixes(), false, false)
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
	sc := &syncContext{cmd: cmd, r: r, cfg: testSyncConfig(), s: testSyncSpinner(cmd)}

	err := syncOne(context.Background(), sc, "login", worktrees, testSyncPrefixes(), false, false)
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
		sc := &syncContext{cmd: cmd, r: r, cfg: testSyncConfig(), s: testSyncSpinner(cmd)}

		err := syncOne(context.Background(), sc, "login", worktrees, testSyncPrefixes(), false, false)
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
		sc := &syncContext{cmd: cmd, r: r, cfg: testSyncConfig(), s: testSyncSpinner(cmd)}

		err := syncOne(context.Background(), sc, "login", worktrees, testSyncPrefixes(), false, false)
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
	sc := &syncContext{cmd: cmd, r: r, cfg: testSyncConfig(), s: testSyncSpinner(cmd)}

	err := syncOne(context.Background(), sc, "login", worktrees, testSyncPrefixes(), false, true)
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
	sc := &syncContext{cmd: cmd, r: r, cfg: testSyncConfig(), s: testSyncSpinner(cmd)}

	err := syncOne(context.Background(), sc, "login", worktrees, testSyncPrefixes(), false, true)
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
			if len(args) >= 1 && args[0] == flagPush {
				return "", errors.New("rejected")
			}
			return "", nil
		},
	}
	sc := &syncContext{cmd: cmd, r: r, cfg: testSyncConfig(), s: testSyncSpinner(cmd)}

	err := syncOne(context.Background(), sc, "login", worktrees, testSyncPrefixes(), false, true)
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

// TestSyncOnePushFailureJSON proves a push failure in JSON mode still emits a
// success envelope (the rebase/merge already succeeded) with push_failed and
// push_error set, instead of collapsing into a bare error envelope.
func TestSyncOnePushFailureJSON(t *testing.T) {
	worktrees := testSyncWorktrees()
	cmd, buf := newTestCmd()
	_ = cmd.Flags().Set(flagJSON, "true")
	r := &mockRunner{
		run: func(_ ...string) (string, error) { return "", nil },
		runInDir: func(_ string, args ...string) (string, error) {
			if len(args) >= 1 && args[0] == cmdRevParse {
				return fakeUpstreamRef, nil
			}
			if len(args) >= 1 && args[0] == flagPush {
				return "", errors.New("rejected")
			}
			return "", nil
		},
	}
	sc := &syncContext{cmd: cmd, r: r, cfg: testSyncConfig(), s: testSyncSpinner(cmd)}

	err := syncOne(context.Background(), sc, "login", worktrees, testSyncPrefixes(), false, true)
	if err != nil {
		t.Fatalf("syncOne: %v, want a success envelope reporting the push failure instead", err)
	}

	_, data := decodeAddEnvelope(t, buf.Bytes())
	worktreesData, ok := data["worktrees"].([]any)
	if !ok || len(worktreesData) != 1 {
		t.Fatalf("worktrees = %v, want 1 entry", data["worktrees"])
	}
	wt, ok := worktreesData[0].(map[string]any)
	if !ok {
		t.Fatalf("worktrees[0] type = %T, want map[string]any", worktreesData[0])
	}
	if wt["synced"] != true {
		t.Errorf("synced = %v, want true (the sync itself succeeded)", wt["synced"])
	}
	if wt["push_failed"] != true {
		t.Errorf("push_failed = %v, want true", wt["push_failed"])
	}
	pushErr, _ := wt["push_error"].(string)
	if !strings.Contains(pushErr, "rejected") {
		t.Errorf("push_error = %q, want it to mention the underlying failure", pushErr)
	}
	if strings.Contains(pushErr, "To fix:") {
		t.Errorf("push_error = %q, should be the raw error without the text-mode hint", pushErr)
	}
}

func TestSyncAllClean(t *testing.T) {
	worktrees := testSyncWorktrees()
	cmd, buf := newTestCmd()
	r := &mockRunner{
		run:      func(_ ...string) (string, error) { return "", nil },
		runInDir: noopRunInDir,
	}
	sc := &syncContext{cmd: cmd, r: r, cfg: testSyncConfig(), s: testSyncSpinner(cmd)}

	err := syncAll(context.Background(), sc, worktrees, testSyncPrefixes(), false, false, false)
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
	sc := &syncContext{cmd: cmd, r: r, cfg: testSyncConfig(), s: testSyncSpinner(cmd)}

	err := syncAll(context.Background(), sc, worktrees, testSyncPrefixes(), false, false, false)
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
	sc := &syncContext{cmd: cmd, r: r, cfg: testSyncConfig(), s: testSyncSpinner(cmd)}

	err := syncAll(context.Background(), sc, worktrees, testSyncPrefixes(), false, true, false)
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
	sc := &syncContext{cmd: cmd, r: r, cfg: testSyncConfig(), s: testSyncSpinner(cmd)}

	err := syncAll(context.Background(), sc, worktrees, testSyncPrefixes(), false, false, true)
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
			if len(args) >= 1 && args[0] == flagPush {
				return "", errors.New("rejected")
			}
			return "", nil
		},
	}
	sc := &syncContext{cmd: cmd, r: r, cfg: testSyncConfig(), s: testSyncSpinner(cmd)}

	err := syncAll(context.Background(), sc, worktrees, testSyncPrefixes(), false, false, true)
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

	sc := &syncContext{cmd: cmd, r: r, res: &syncResult{}}
	wt := resolver.WorktreeInfo{Branch: branchFeature, Path: pathWtFeatureLogin}
	syncWorktree(context.Background(), sc, branchMain, wt, false, false)

	if sc.res.synced != 1 {
		t.Errorf("synced = %d, want 1", sc.res.synced)
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

		sc := &syncContext{cmd: cmd, r: r, res: &syncResult{}}
		wt := resolver.WorktreeInfo{Branch: branchFeature, Path: pathWtFeatureLogin}
		syncWorktree(context.Background(), sc, branchMain, wt, false, false)

		if sc.res.skippedDirty != 1 {
			t.Errorf("skippedDirty = %d, want 1", sc.res.skippedDirty)
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

		sc := &syncContext{cmd: cmd, r: r, res: &syncResult{}}
		wt := resolver.WorktreeInfo{Branch: branchFeature, Path: pathWtFeatureLogin}
		syncWorktree(context.Background(), sc, branchMain, wt, false, false)

		if sc.res.skippedDirty != 1 {
			t.Errorf("skippedDirty = %d, want 1", sc.res.skippedDirty)
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

	sc := &syncContext{cmd: cmd, r: r, res: &syncResult{}}
	wt := resolver.WorktreeInfo{Branch: branchFeature, Path: pathWtFeatureLogin}
	syncWorktree(context.Background(), sc, branchMain, wt, false, false)

	if sc.res.failed != 1 {
		t.Errorf("failed = %d, want 1", sc.res.failed)
	}
	if len(sc.res.failures) != 1 {
		t.Errorf("failures = %d, want 1", len(sc.res.failures))
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

	sc := &syncContext{cmd: cmd, r: r, res: &syncResult{}}
	wt := resolver.WorktreeInfo{Branch: branchFeature, Path: pathWtFeatureLogin}
	syncWorktree(context.Background(), sc, branchMain, wt, true, false)

	if sc.res.failed != 1 {
		t.Errorf("failed = %d, want 1", sc.res.failed)
	}
	if len(sc.res.failures) > 0 && !strings.Contains(sc.res.failures[0], "merge") {
		t.Errorf("failure message = %q, want 'merge'", sc.res.failures[0])
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

func TestSyncOneDryRun(t *testing.T) {
	worktrees := testSyncWorktrees()
	cmd, buf := newTestCmd()
	rebaseCalled := false
	r := &mockRunner{
		run: func(_ ...string) (string, error) { return "", nil },
		runInDir: func(_ string, args ...string) (string, error) {
			if len(args) >= 1 && args[0] == cmdRebase {
				rebaseCalled = true
			}
			return "", nil
		},
	}
	sc := &syncContext{cmd: cmd, r: r, cfg: testSyncConfig(), s: testSyncSpinner(cmd), dryRun: true}

	if err := syncOne(context.Background(), sc, "login", worktrees, testSyncPrefixes(), false, true); err != nil {
		t.Fatalf("syncOne: %v", err)
	}
	if rebaseCalled {
		t.Error("rebase must not be called in dry-run mode")
	}
	out := buf.String()
	if !strings.Contains(out, "[dry-run]") {
		t.Errorf("output = %q, want '[dry-run]' prefix", out)
	}
	if strings.Contains(out, "Rebased") {
		t.Errorf("output = %q, must not contain 'Rebased' in dry-run", out)
	}
}

func TestSyncAllDryRun(t *testing.T) {
	worktrees := testSyncWorktrees()
	cmd, buf := newTestCmd()
	r := &mockRunner{
		run:      func(_ ...string) (string, error) { return "", nil },
		runInDir: noopRunInDir,
	}
	sc := &syncContext{cmd: cmd, r: r, cfg: testSyncConfig(), s: testSyncSpinner(cmd), dryRun: true}

	if err := syncAll(context.Background(), sc, worktrees, testSyncPrefixes(), false, false, true); err != nil {
		t.Fatalf("syncAll: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "[dry-run]") {
		t.Errorf("output = %q, want '[dry-run]' lines", out)
	}
	if strings.Contains(out, "Rebased") {
		t.Errorf("output = %q, must not contain 'Rebased' in dry-run", out)
	}
}

func TestSyncWorktreeSkipWarningOnStderr(t *testing.T) {
	var outBuf, errBuf bytes.Buffer
	cmd := &cobra.Command{}
	cmd.Flags().Bool(flagNoColor, true, "")
	cmd.Flags().Bool(flagJSON, false, "")
	cmd.SetOut(&outBuf)
	cmd.SetErr(&errBuf)

	r := &mockRunner{
		run: func(_ ...string) (string, error) { return "", nil },
		runInDir: func(_ string, _ ...string) (string, error) {
			return "", errGitFailed
		},
	}

	sc := &syncContext{cmd: cmd, r: r, res: &syncResult{}}
	wt := resolver.WorktreeInfo{Branch: branchFeature, Path: pathWtFeatureLogin}
	syncWorktree(context.Background(), sc, branchMain, wt, false, false)

	if strings.Contains(outBuf.String(), "Warning") {
		t.Errorf("warning leaked to stdout: %q", outBuf.String())
	}
	if !strings.Contains(errBuf.String(), "Warning") {
		t.Errorf("stderr = %q, want warning", errBuf.String())
	}
}

func TestSyncOneJSON(t *testing.T) {
	worktrees := testSyncWorktrees()
	cmd, buf := newTestCmd()
	_ = cmd.Flags().Set(flagJSON, "true")
	r := &mockRunner{
		run:      func(_ ...string) (string, error) { return "", nil },
		runInDir: noopRunInDir,
	}
	sc := &syncContext{cmd: cmd, r: r, cfg: testSyncConfig(), s: testSyncSpinner(cmd)}

	err := syncOne(context.Background(), sc, "login", worktrees, testSyncPrefixes(), false, false)
	if err != nil {
		t.Fatalf("syncOne: %v", err)
	}

	env, data := decodeAddEnvelope(t, buf.Bytes())
	if env.Command != wantSyncCommand {
		t.Errorf("command = %q, want %q", env.Command, wantSyncCommand)
	}
	if data["all"] != false {
		t.Errorf("all = %v, want false", data["all"])
	}
	worktreesJSON, ok := data["worktrees"].([]any)
	if !ok || len(worktreesJSON) != 1 {
		t.Fatalf("worktrees = %#v, want 1 element", data["worktrees"])
	}
	wt0, ok := worktreesJSON[0].(map[string]any)
	if !ok {
		t.Fatalf("worktrees[0] type = %T", worktreesJSON[0])
	}
	if wt0["synced"] != true {
		t.Errorf("worktrees[0].synced = %v, want true", wt0["synced"])
	}
	summary, ok := data["summary"].(map[string]any)
	if !ok {
		t.Fatalf("summary type = %T", data["summary"])
	}
	if summary["synced"] != float64(1) {
		t.Errorf("summary.synced = %v, want 1", summary["synced"])
	}
}

func TestSyncOneDryRunJSON(t *testing.T) {
	worktrees := testSyncWorktrees()
	cmd, buf := newTestCmd()
	_ = cmd.Flags().Set(flagJSON, "true")
	r := &mockRunner{
		run:      func(_ ...string) (string, error) { return "", nil },
		runInDir: noopRunInDir,
	}
	sc := &syncContext{cmd: cmd, r: r, cfg: testSyncConfig(), s: testSyncSpinner(cmd), dryRun: true}

	err := syncOne(context.Background(), sc, "login", worktrees, testSyncPrefixes(), false, true)
	if err != nil {
		t.Fatalf("syncOne: %v", err)
	}

	_, data := decodeAddEnvelope(t, buf.Bytes())
	if data["dry_run"] != true {
		t.Errorf("dry_run = %v, want true", data["dry_run"])
	}
	worktreesJSON, ok := data["worktrees"].([]any)
	if !ok || len(worktreesJSON) != 1 {
		t.Fatalf("worktrees = %#v, want 1 element", data["worktrees"])
	}
	wt0, ok := worktreesJSON[0].(map[string]any)
	if !ok {
		t.Fatalf("worktrees[0] type = %T", worktreesJSON[0])
	}
	if wt0["planned"] != true {
		t.Errorf("worktrees[0].planned = %v, want true", wt0["planned"])
	}
	if wt0["synced"] != false {
		t.Errorf("worktrees[0].synced = %v, want false", wt0["synced"])
	}
}

func TestSyncAllJSON(t *testing.T) {
	worktrees := testSyncWorktrees()
	cmd, buf := newTestCmd()
	_ = cmd.Flags().Set(flagJSON, "true")
	r := &mockRunner{
		run:      func(_ ...string) (string, error) { return "", nil },
		runInDir: noopRunInDir,
	}
	sc := &syncContext{cmd: cmd, r: r, cfg: testSyncConfig(), s: testSyncSpinner(cmd)}

	err := syncAll(context.Background(), sc, worktrees, testSyncPrefixes(), false, false, false)
	if err != nil {
		t.Fatalf(fatalSyncAll, err)
	}

	env, data := decodeAddEnvelope(t, buf.Bytes())
	if env.Command != wantSyncCommand {
		t.Errorf("command = %q, want %q", env.Command, wantSyncCommand)
	}
	if data["all"] != true {
		t.Errorf("all = %v, want true", data["all"])
	}
	worktreesJSON, ok := data["worktrees"].([]any)
	if !ok || len(worktreesJSON) != 2 {
		t.Fatalf("worktrees = %#v, want 2 elements", data["worktrees"])
	}
	summary, ok := data["summary"].(map[string]any)
	if !ok {
		t.Fatalf("summary type = %T", data["summary"])
	}
	synced, ok := summary["synced"].(float64)
	if !ok {
		t.Fatalf("summary.synced type = %T", summary["synced"])
	}
	skippedDirty, ok := summary["skipped_dirty"].(float64)
	if !ok {
		t.Fatalf("summary.skipped_dirty type = %T", summary["skipped_dirty"])
	}
	failed, ok := summary["failed"].(float64)
	if !ok {
		t.Fatalf("summary.failed type = %T", summary["failed"])
	}
	if total := synced + skippedDirty + failed; total != float64(len(worktreesJSON)) {
		t.Errorf("summary counters sum = %v, want %d", total, len(worktreesJSON))
	}
}

func TestSyncAllDryRunJSON(t *testing.T) {
	worktrees := testSyncWorktrees()
	cmd, buf := newTestCmd()
	_ = cmd.Flags().Set(flagJSON, "true")
	r := &mockRunner{
		run:      func(_ ...string) (string, error) { return "", nil },
		runInDir: noopRunInDir,
	}
	sc := &syncContext{cmd: cmd, r: r, cfg: testSyncConfig(), s: testSyncSpinner(cmd), dryRun: true}

	err := syncAll(context.Background(), sc, worktrees, testSyncPrefixes(), false, false, true)
	if err != nil {
		t.Fatalf(fatalSyncAll, err)
	}

	_, data := decodeAddEnvelope(t, buf.Bytes())
	worktreesJSON, ok := data["worktrees"].([]any)
	if !ok || len(worktreesJSON) == 0 {
		t.Fatalf("worktrees = %#v, want non-empty", data["worktrees"])
	}
	for i, w := range worktreesJSON {
		wm, ok := w.(map[string]any)
		if !ok {
			t.Fatalf("worktrees[%d] type = %T", i, w)
		}
		if wm["planned"] != true {
			t.Errorf("worktrees[%d].planned = %v, want true", i, wm["planned"])
		}
	}
}

// TestSyncWorktreeJSONSilencesSkipWarning covers the isJSON guard on the
// per-worktree skip/warning prints inside syncWorktree's switch.
func TestSyncWorktreeJSONSilencesSkipWarning(t *testing.T) {
	var outBuf, errBuf bytes.Buffer
	cmd := &cobra.Command{}
	cmd.Flags().Bool(flagNoColor, true, "")
	cmd.Flags().Bool(flagJSON, true, "")
	cmd.SetOut(&outBuf)
	cmd.SetErr(&errBuf)

	r := &mockRunner{
		run: func(_ ...string) (string, error) { return "", nil },
		runInDir: func(_ string, args ...string) (string, error) {
			if len(args) >= 1 && args[0] == cmdStatus {
				return dirtyOutput, nil
			}
			return "", nil
		},
	}

	sc := &syncContext{cmd: cmd, r: r, res: &syncResult{}}
	wt := resolver.WorktreeInfo{Branch: branchFeature, Path: pathWtFeatureLogin}
	syncWorktree(context.Background(), sc, branchMain, wt, false, false)

	if sc.res.skippedDirty != 1 {
		t.Errorf("skippedDirty = %d, want 1", sc.res.skippedDirty)
	}
	if outBuf.Len() != 0 {
		t.Errorf("stdout = %q, want empty in JSON mode", outBuf.String())
	}
	if errBuf.Len() != 0 {
		t.Errorf("stderr = %q, want empty in JSON mode", errBuf.String())
	}
	if len(sc.jsonResults) != 1 || sc.jsonResults[0].SkipReason != "dirty" {
		t.Errorf("jsonResults = %#v, want 1 result with SkipReason \"dirty\"", sc.jsonResults)
	}
}

func TestSyncAllCancelledSkipsSummary(t *testing.T) {
	worktrees := testSyncWorktrees()
	cmd, buf := newTestCmd()
	r := &mockRunner{
		run:      func(_ ...string) (string, error) { return "", nil },
		runInDir: noopRunInDir,
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // pre-cancel so syncAll exits without goroutines doing work
	sc := &syncContext{cmd: cmd, r: r, cfg: testSyncConfig(), s: testSyncSpinner(cmd)}

	err := syncAll(ctx, sc, worktrees, testSyncPrefixes(), false, false, false)
	if !errors.Is(err, context.Canceled) {
		t.Errorf("syncAll with cancelled ctx: err = %v, want context.Canceled", err)
	}
	if strings.Contains(buf.String(), "worktree(s)") {
		t.Errorf("partial summary should not be printed on cancellation: %q", buf.String())
	}
}

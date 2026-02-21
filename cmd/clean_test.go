package cmd

import (
	"bytes"
	"errors"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/lugassawan/rimba/internal/config"
	"github.com/lugassawan/rimba/internal/operations"
	"github.com/spf13/cobra"
)

func TestCleanMergedFetchFails(t *testing.T) {
	worktreeOut := cleanMergedWorktreeOut()
	cmd, buf := newCleanMergedCmd()
	_ = cmd.Flags().Set(flagForce, "true")

	var mergedRef string
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			if len(args) >= 2 && args[1] == cmdShowToplevel {
				return t.TempDir(), nil
			}
			if args[0] == cmdSymbolicRef {
				return refsRemotesOriginMain, nil
			}
			if args[0] == cmdFetch {
				return "", errors.New("no remote")
			}
			if args[0] == cmdBranch {
				mergedRef = args[len(args)-1]
				return "", nil
			}
			if args[0] == cmdWorktreeTest {
				return worktreeOut, nil
			}
			return "", nil
		},
		runInDir: noopRunInDir,
	}

	err := cleanMerged(cmd, r)
	if err != nil {
		t.Fatalf(fatalCleanMerged, err)
	}
	out := buf.String()
	if !strings.Contains(out, "Warning: fetch failed") {
		t.Errorf("expected fetch warning, got %q", out)
	}
	if mergedRef != branchMain {
		t.Errorf("merged ref = %q, want %q (local fallback)", mergedRef, branchMain)
	}
}

func TestCleanMergedFetchSucceeds(t *testing.T) {
	worktreeOut := cleanMergedWorktreeOut()
	cmd, buf := newCleanMergedCmd()
	_ = cmd.Flags().Set(flagForce, "true")

	var mergedRef string
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			if len(args) >= 2 && args[1] == cmdShowToplevel {
				return t.TempDir(), nil
			}
			if args[0] == cmdSymbolicRef {
				return refsRemotesOriginMain, nil
			}
			if args[0] == cmdFetch {
				return "", nil // fetch succeeds
			}
			if args[0] == cmdBranch {
				mergedRef = args[len(args)-1]
				return "", nil
			}
			if args[0] == cmdWorktreeTest {
				return worktreeOut, nil
			}
			return "", nil
		},
		runInDir: noopRunInDir,
	}

	err := cleanMerged(cmd, r)
	if err != nil {
		t.Fatalf(fatalCleanMerged, err)
	}
	out := buf.String()
	if strings.Contains(out, "Warning: fetch failed") {
		t.Errorf("unexpected fetch warning in output: %q", out)
	}
	wantRef := "origin/" + branchMain
	if mergedRef != wantRef {
		t.Errorf("merged ref = %q, want %q", mergedRef, wantRef)
	}
}

func TestPrintMergedCandidates(t *testing.T) {
	cmd, buf := newTestCmd()
	candidates := []operations.CleanCandidate{
		{Path: pathWtDone, Branch: branchDone},
		{Path: "/wt/bugfix-old", Branch: "bugfix/old"},
	}
	printMergedCandidates(cmd, candidates)

	out := buf.String()
	if !strings.Contains(out, "Merged worktrees:") {
		t.Errorf("output missing header: %q", out)
	}
	if !strings.Contains(out, "done") {
		t.Errorf("output missing task 'done': %q", out)
	}
	if !strings.Contains(out, "old") {
		t.Errorf("output missing task 'old': %q", out)
	}
}

func TestConfirmRemoval(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{"yes_short", "y\n", true},
		{"yes_full", "yes\n", true},
		{"no", "n\n", false},
		{"empty", "\n", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd, _ := newTestCmd()
			cmd.SetIn(strings.NewReader(tt.input))
			got := confirmRemoval(cmd, 2, "merged")
			if got != tt.want {
				t.Errorf("confirmRemoval(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func newCleanPruneCmd() (*cobra.Command, *bytes.Buffer) {
	cmd, buf := newTestCmd()
	cmd.Flags().Bool(flagDryRun, false, "")
	cmd.Flags().Bool(flagMerged, false, "")
	return cmd, buf
}

func TestCleanPruneSuccess(t *testing.T) {
	cmd, buf := newCleanPruneCmd()
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			if len(args) >= 1 && args[0] == cmdWorktreeTest {
				return "Removing worktrees/stale", nil
			}
			return "", nil
		},
		runInDir: noopRunInDir,
	}

	err := cleanPrune(cmd, r)
	if err != nil {
		t.Fatalf(fatalCleanPrune, err)
	}
	if !strings.Contains(buf.String(), "Removing worktrees/stale") {
		t.Errorf("output = %q, want prune output", buf.String())
	}
}

func TestCleanPruneDryRunEmpty(t *testing.T) {
	cmd, buf := newCleanPruneCmd()
	_ = cmd.Flags().Set(flagDryRun, "true")
	r := &mockRunner{
		run:      func(_ ...string) (string, error) { return "", nil },
		runInDir: noopRunInDir,
	}

	err := cleanPrune(cmd, r)
	if err != nil {
		t.Fatalf(fatalCleanPrune, err)
	}
	if !strings.Contains(buf.String(), "Nothing to prune") {
		t.Errorf("output = %q, want 'Nothing to prune'", buf.String())
	}
}

func TestCleanPruneNoDryRunEmpty(t *testing.T) {
	cmd, buf := newCleanPruneCmd()
	r := &mockRunner{
		run:      func(_ ...string) (string, error) { return "", nil },
		runInDir: noopRunInDir,
	}

	err := cleanPrune(cmd, r)
	if err != nil {
		t.Fatalf(fatalCleanPrune, err)
	}
	if !strings.Contains(buf.String(), "Pruned stale worktree references") {
		t.Errorf("output = %q, want 'Pruned stale worktree references'", buf.String())
	}
}

func TestCleanPruneError(t *testing.T) {
	cmd, _ := newCleanPruneCmd()
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			if len(args) >= 1 && args[0] == cmdWorktreeTest {
				return "", errGitFailed
			}
			return "", nil
		},
		runInDir: noopRunInDir,
	}

	err := cleanPrune(cmd, r)
	if err == nil {
		t.Fatal("expected error from prune failure")
	}
}

func cleanMergedTestRunner(t *testing.T, mergedOut, worktreeOut string) *mockRunner {
	t.Helper()
	dir := t.TempDir()
	cfg := &config.Config{DefaultSource: branchMain}
	_ = config.Save(filepath.Join(dir, config.FileName), cfg)

	return &mockRunner{
		run: func(args ...string) (string, error) {
			if len(args) >= 2 && args[1] == cmdShowToplevel {
				return dir, nil
			}
			if len(args) >= 1 && args[0] == cmdBranch {
				return mergedOut, nil
			}
			if len(args) >= 1 && args[0] == cmdWorktreeTest {
				return worktreeOut, nil
			}
			if len(args) >= 1 && args[0] == cmdFetch {
				return "", errors.New("no remote")
			}
			return "", nil
		},
		runInDir: noopRunInDir,
	}
}

func newCleanMergedCmd() (*cobra.Command, *bytes.Buffer) {
	cmd, buf := newTestCmd()
	cmd.Flags().Bool(flagDryRun, false, "")
	cmd.Flags().Bool(flagForce, false, "")
	cmd.Flags().Bool(flagMerged, false, "")
	return cmd, buf
}

func cleanMergedWorktreeOut() string {
	return strings.Join([]string{
		wtRepo,
		"HEAD abc123",
		"branch refs/heads/main",
		"",
		wtDone,
		"HEAD def456",
		branchRefPrefix + branchDone,
		"",
	}, "\n")
}

func TestCleanMergedNoCandidates(t *testing.T) {
	worktreeOut := cleanMergedWorktreeOut()
	cmd, buf := newCleanMergedCmd()
	r := cleanMergedTestRunner(t, "", worktreeOut)

	err := cleanMerged(cmd, r)
	if err != nil {
		t.Fatalf(fatalCleanMerged, err)
	}
	if !strings.Contains(buf.String(), "No merged worktrees found") {
		t.Errorf("output = %q, want 'No merged worktrees found'", buf.String())
	}
}

func TestCleanMergedDryRun(t *testing.T) {
	worktreeOut := cleanMergedWorktreeOut()
	cmd, buf := newCleanMergedCmd()
	_ = cmd.Flags().Set(flagDryRun, "true")
	r := cleanMergedTestRunner(t, "  "+branchDone, worktreeOut)

	err := cleanMerged(cmd, r)
	if err != nil {
		t.Fatalf(fatalCleanMerged, err)
	}
	out := buf.String()
	if !strings.Contains(out, "Merged worktrees:") {
		t.Errorf("output = %q, want 'Merged worktrees:'", out)
	}
	if strings.Contains(out, "Cleaned") {
		t.Errorf("dry-run should not show 'Cleaned'")
	}
}

func TestCleanMergedAbort(t *testing.T) {
	worktreeOut := cleanMergedWorktreeOut()
	cmd, buf := newCleanMergedCmd()
	cmd.SetIn(strings.NewReader("n\n"))
	r := cleanMergedTestRunner(t, "  "+branchDone, worktreeOut)

	err := cleanMerged(cmd, r)
	if err != nil {
		t.Fatalf(fatalCleanMerged, err)
	}
	if !strings.Contains(buf.String(), "Aborted") {
		t.Errorf("output = %q, want 'Aborted'", buf.String())
	}
}

func TestCleanMergedForce(t *testing.T) {
	worktreeOut := cleanMergedWorktreeOut()
	cmd, buf := newCleanMergedCmd()
	_ = cmd.Flags().Set(flagForce, "true")
	r := cleanMergedTestRunner(t, "  "+branchDone, worktreeOut)

	err := cleanMerged(cmd, r)
	if err != nil {
		t.Fatalf(fatalCleanMerged, err)
	}
	out := buf.String()
	if !strings.Contains(out, "Cleaned") {
		t.Errorf("output = %q, want 'Cleaned'", out)
	}
}

func TestCleanMergedResolveError(t *testing.T) {
	cmd, _ := newCleanMergedCmd()
	r := &mockRunner{
		run:      func(_ ...string) (string, error) { return "", errGitFailed },
		runInDir: noopRunInDir,
	}

	err := cleanMerged(cmd, r)
	if err == nil {
		t.Fatal("expected error from resolveMainBranch failure")
	}
}

func newCleanStaleCmd() (*cobra.Command, *bytes.Buffer) {
	cmd, buf := newTestCmd()
	cmd.Flags().Bool(flagDryRun, false, "")
	cmd.Flags().Bool(flagForce, false, "")
	cmd.Flags().Bool(flagStale, false, "")
	cmd.Flags().Int(flagStaleDays, defaultStaleDays, "")
	return cmd, buf
}

func TestCleanStaleDryRun(t *testing.T) {
	oldTimestamp := strconv.FormatInt(time.Now().Add(-30*24*time.Hour).Unix(), 10)
	cmd, buf := newCleanStaleCmd()
	_ = cmd.Flags().Set(flagDryRun, "true")

	dir := t.TempDir()
	cfg := &config.Config{DefaultSource: branchMain}
	_ = config.Save(filepath.Join(dir, config.FileName), cfg)

	r := &mockRunner{
		run: func(args ...string) (string, error) {
			if len(args) >= 2 && args[1] == cmdShowToplevel {
				return dir, nil
			}
			if args[0] == cmdWorktreeTest && args[1] == cmdList {
				return strings.Join([]string{
					wtRepo, headABC123, branchRefMain, "",
					wtFeatureLogin, headDEF456, branchRefFeatureLogin, "",
				}, "\n"), nil
			}
			if args[0] == cmdLog {
				return oldTimestamp + "\tcommit", nil
			}
			return "", nil
		},
		runInDir: noopRunInDir,
	}

	err := cleanStale(cmd, r)
	if err != nil {
		t.Fatalf("cleanStale: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "Stale worktrees:") {
		t.Errorf("output = %q, want 'Stale worktrees:'", out)
	}
	if strings.Contains(out, "Cleaned") {
		t.Errorf("dry-run should not show 'Cleaned'")
	}
}

func TestCleanStaleNoCandidates(t *testing.T) {
	recentTimestamp := strconv.FormatInt(time.Now().Add(-1*time.Hour).Unix(), 10)
	cmd, buf := newCleanStaleCmd()

	dir := t.TempDir()
	cfg := &config.Config{DefaultSource: branchMain}
	_ = config.Save(filepath.Join(dir, config.FileName), cfg)

	r := &mockRunner{
		run: func(args ...string) (string, error) {
			if len(args) >= 2 && args[1] == cmdShowToplevel {
				return dir, nil
			}
			if args[0] == cmdWorktreeTest && args[1] == cmdList {
				return strings.Join([]string{
					wtRepo, headABC123, branchRefMain, "",
					wtFeatureLogin, headDEF456, branchRefFeatureLogin, "",
				}, "\n"), nil
			}
			if args[0] == cmdLog {
				return recentTimestamp + "\tcommit", nil
			}
			return "", nil
		},
		runInDir: noopRunInDir,
	}

	err := cleanStale(cmd, r)
	if err != nil {
		t.Fatalf("cleanStale: %v", err)
	}
	if !strings.Contains(buf.String(), "No stale worktrees found") {
		t.Errorf("output = %q, want 'No stale worktrees found'", buf.String())
	}
}

func TestCleanStaleAbort(t *testing.T) {
	oldTimestamp := strconv.FormatInt(time.Now().Add(-30*24*time.Hour).Unix(), 10)
	cmd, buf := newCleanStaleCmd()
	cmd.SetIn(strings.NewReader("n\n"))

	dir := t.TempDir()
	cfg := &config.Config{DefaultSource: branchMain}
	_ = config.Save(filepath.Join(dir, config.FileName), cfg)

	r := &mockRunner{
		run: func(args ...string) (string, error) {
			if len(args) >= 2 && args[1] == cmdShowToplevel {
				return dir, nil
			}
			if args[0] == cmdWorktreeTest && args[1] == cmdList {
				return strings.Join([]string{
					wtRepo, headABC123, branchRefMain, "",
					wtFeatureLogin, headDEF456, branchRefFeatureLogin, "",
				}, "\n"), nil
			}
			if args[0] == cmdLog {
				return oldTimestamp + "\tcommit", nil
			}
			return "", nil
		},
		runInDir: noopRunInDir,
	}

	err := cleanStale(cmd, r)
	if err != nil {
		t.Fatalf("cleanStale: %v", err)
	}
	if !strings.Contains(buf.String(), "Aborted") {
		t.Errorf("output = %q, want 'Aborted'", buf.String())
	}
}

func TestCleanStaleResolveError(t *testing.T) {
	cmd, _ := newCleanStaleCmd()
	r := &mockRunner{
		run:      func(_ ...string) (string, error) { return "", errGitFailed },
		runInDir: noopRunInDir,
	}

	err := cleanStale(cmd, r)
	if err == nil {
		t.Fatal("expected error from resolveMainBranch failure")
	}
}

func TestCleanMergedFindCandidatesError(t *testing.T) {
	cmd, _ := newCleanMergedCmd()
	// resolveMainBranch succeeds but findMergedCandidates fails (MergedBranches error)
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			if args[0] == cmdSymbolicRef {
				return refsRemotesOriginMain, nil
			}
			if args[0] == cmdFetch {
				return "", errors.New("no remote")
			}
			if args[0] == cmdBranch {
				return "", errGitFailed
			}
			return "", nil
		},
		runInDir: noopRunInDir,
	}

	err := cleanMerged(cmd, r)
	if err == nil {
		t.Fatal("expected error from findMergedCandidates failure")
	}
}

func TestCleanStaleFindCandidatesError(t *testing.T) {
	cmd, _ := newCleanStaleCmd()
	// resolveMainBranch succeeds but findStaleCandidates fails (ListWorktrees error)
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			if args[0] == cmdSymbolicRef {
				return refsRemotesOriginMain, nil
			}
			if args[0] == cmdWorktreeTest {
				return "", errGitFailed
			}
			return "", nil
		},
		runInDir: noopRunInDir,
	}

	err := cleanStale(cmd, r)
	if err == nil {
		t.Fatal("expected error from findStaleCandidates failure")
	}
}

func TestCleanStaleForce(t *testing.T) {
	oldTimestamp := strconv.FormatInt(time.Now().Add(-30*24*time.Hour).Unix(), 10)
	cmd, buf := newCleanStaleCmd()
	_ = cmd.Flags().Set(flagForce, "true")

	dir := t.TempDir()
	cfg := &config.Config{DefaultSource: branchMain}
	_ = config.Save(filepath.Join(dir, config.FileName), cfg)

	r := &mockRunner{
		run: func(args ...string) (string, error) {
			if len(args) >= 2 && args[1] == cmdShowToplevel {
				return dir, nil
			}
			if args[0] == cmdWorktreeTest && args[1] == cmdList {
				return strings.Join([]string{
					wtRepo, headABC123, branchRefMain, "",
					wtFeatureLogin, headDEF456, branchRefFeatureLogin, "",
				}, "\n"), nil
			}
			if args[0] == cmdLog {
				return oldTimestamp + "\tcommit", nil
			}
			return "", nil
		},
		runInDir: noopRunInDir,
	}

	err := cleanStale(cmd, r)
	if err != nil {
		t.Fatalf("cleanStale: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "Cleaned") {
		t.Errorf("output = %q, want 'Cleaned'", out)
	}
}

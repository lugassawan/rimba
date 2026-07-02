package git

import (
	"context"
	"errors"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

const (
	fakeDir          = "/fake"
	fakeRepoDir      = "/repo"
	fakeGitDir       = ".git"
	fakePath         = "/some/path"
	errNotARepo      = "not a git repo"
	flagForceD       = "-D"
	flagDryRun       = "--dry-run"
	branchOld        = "old-branch"
	branchNew        = "new-branch"
	pruneOutput      = "Pruning worktree"
	errNotAGitRepo   = "not a git repository"
	errContainsFmt   = "error = %q, want it to contain %q"
	errExpectedInFmt = "expected %s in args %v"
	fatalDefaultFmt  = "DefaultBranch: %v"
	errBranchWantFmt = "branch = %q, want %q"
	flagGitCommonDir = "--git-common-dir"
	flagShowToplevel = "--show-toplevel"
	fakeTip          = "tip789"
	fakeDiff         = "some diff"
	fakeLog          = "fake log"
)

// mockRunner implements Runner with configurable closures for testing.
type mockRunner struct {
	run      func(args ...string) (string, error)
	runInDir func(dir string, args ...string) (string, error)
}

func (m *mockRunner) Run(_ context.Context, args ...string) (string, error) {
	return m.run(args...)
}

func (m *mockRunner) RunInDir(_ context.Context, dir string, args ...string) (string, error) {
	return m.runInDir(dir, args...)
}

// assertContains fails the test if err's message does not contain substr.
func assertContains(t *testing.T, err error, substr string) {
	t.Helper()
	if !strings.Contains(err.Error(), substr) {
		t.Errorf(errContainsFmt, err, substr)
	}
}

func TestParseCount(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  int
	}{
		{"zero", "0", 0},
		{"positive", "42", 42},
		{"trailing_non_digit", "12abc", 12},
		{"empty", "", 0},
		{"non_digit_only", "abc", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var v int
			parseCount(tt.input, &v)
			if v != tt.want {
				t.Errorf("parseCount(%q) set *v = %d, want %d", tt.input, v, tt.want)
			}
		})
	}
}

func TestAheadBehind(t *testing.T) {
	tests := []struct {
		name       string
		out        string
		err        error
		wantAhead  int
		wantBehind int
	}{
		{"error_returns_zeros", "", errors.New(errNotARepo), 0, 0},
		{"malformed_single_field", "5", nil, 0, 0},
		{"valid_counts", "3\t7", nil, 7, 3},
		{"zeros", "0\t0", nil, 0, 0},
		{"empty_output", "", nil, 0, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &mockRunner{
				runInDir: func(_ string, _ ...string) (string, error) {
					return tt.out, tt.err
				},
			}
			ahead, behind, err := AheadBehind(context.Background(), r, fakeDir)
			if err != nil {
				t.Fatalf("AheadBehind returned error: %v", err)
			}
			if ahead != tt.wantAhead {
				t.Errorf("ahead = %d, want %d", ahead, tt.wantAhead)
			}
			if behind != tt.wantBehind {
				t.Errorf("behind = %d, want %d", behind, tt.wantBehind)
			}
		})
	}
}

func TestMergedBranches(t *testing.T) {
	r := &mockRunner{
		run: func(_ ...string) (string, error) {
			return "  feature/done\n* main\n+ bugfix/old", nil
		},
	}

	branches, err := MergedBranches(context.Background(), r, branchMain)
	if err != nil {
		t.Fatalf("MergedBranches: %v", err)
	}

	want := []string{"feature/done", "main", "bugfix/old"}
	if len(branches) != len(want) {
		t.Fatalf("got %d branches, want %d: %v", len(branches), len(want), branches)
	}
	for i, w := range want {
		if branches[i] != w {
			t.Errorf("branches[%d] = %q, want %q", i, branches[i], w)
		}
	}
}

func TestMergedBranchesError(t *testing.T) {
	r := &mockRunner{
		run: func(_ ...string) (string, error) {
			return "", errors.New(errNotARepo)
		},
	}

	_, err := MergedBranches(context.Background(), r, branchMain)
	if err == nil {
		t.Fatal("expected error from MergedBranches")
	}
}

func TestMergedBranchesEmpty(t *testing.T) {
	r := &mockRunner{
		run: func(_ ...string) (string, error) {
			return "", nil
		},
	}

	branches, err := MergedBranches(context.Background(), r, branchMain)
	if err != nil {
		t.Fatalf("MergedBranches: %v", err)
	}
	if len(branches) != 0 {
		t.Errorf("expected no branches, got %v", branches)
	}
}

func TestDeleteBranchForce(t *testing.T) {
	var captured []string
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			captured = args
			return "", nil
		},
	}

	if err := DeleteBranch(context.Background(), r, branchOld, true); err != nil {
		t.Fatalf("DeleteBranch: %v", err)
	}

	if len(captured) != 3 || captured[1] != flagForceD {
		t.Errorf("expected flag %s, got args %v", flagForceD, captured)
	}
}

func TestDeleteBranchNotFoundIsIdempotent(t *testing.T) {
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			return "", errors.New("error: branch 'gone' not found.")
		},
	}
	if err := DeleteBranch(context.Background(), r, "gone", false); err != nil {
		t.Fatalf("expected nil for already-gone branch, got: %v", err)
	}
}

func TestDeleteBranchOtherErrorPropagates(t *testing.T) {
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			return "", errors.New("error: Cannot delete branch 'main' checked out at '/repo'")
		},
	}
	if err := DeleteBranch(context.Background(), r, "main", false); err == nil {
		t.Fatal("expected non-nil error for checked-out branch")
	}
}

func TestDeleteBranchNotFoundWithoutBranchKeywordPropagates(t *testing.T) {
	// "not found" alone (no "branch '") must NOT be swallowed — tight substring match.
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			return "", errors.New("ref not found")
		},
	}
	if err := DeleteBranch(context.Background(), r, "gone", false); err == nil {
		t.Fatal("expected non-nil error when 'branch \\'' is absent from the message")
	}
}

func TestRenameBranch(t *testing.T) {
	var captured []string
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			captured = args
			return "", nil
		},
	}

	if err := RenameBranch(context.Background(), r, branchOld, branchNew); err != nil {
		t.Fatalf("RenameBranch: %v", err)
	}

	want := []string{"branch", "-m", branchOld, branchNew}
	if len(captured) != len(want) {
		t.Fatalf("args = %v, want %v", captured, want)
	}
	for i, w := range want {
		if captured[i] != w {
			t.Errorf("args[%d] = %q, want %q", i, captured[i], w)
		}
	}
}

func TestIsDirtyError(t *testing.T) {
	r := &mockRunner{
		runInDir: func(_ string, _ ...string) (string, error) {
			return "", errors.New(errNotARepo)
		},
	}

	_, err := IsDirty(context.Background(), r, fakeDir)
	if err == nil {
		t.Fatal("expected error from IsDirty")
	}
	assertContains(t, err, errNotARepo)
}

func TestPruneNormal(t *testing.T) {
	var captured []string
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			captured = args
			return "", nil
		},
	}

	if _, err := Prune(context.Background(), r, false); err != nil {
		t.Fatalf("Prune: %v", err)
	}
	if slices.Contains(captured, flagDryRun) {
		t.Error("--dry-run should not be present when dryRun=false")
	}
}

func TestPruneDryRun(t *testing.T) {
	var captured []string
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			captured = args
			return pruneOutput, nil
		},
	}

	out, err := Prune(context.Background(), r, true)
	if err != nil {
		t.Fatalf("Prune: %v", err)
	}
	if out != pruneOutput {
		t.Errorf("output = %q, want %q", out, pruneOutput)
	}
	if !slices.Contains(captured, flagDryRun) {
		t.Errorf(errExpectedInFmt, flagDryRun, captured)
	}
}

func TestPruneErrorWrapping(t *testing.T) {
	r := &mockRunner{
		run: func(_ ...string) (string, error) {
			return "", errors.New("git failed")
		},
	}

	_, err := Prune(context.Background(), r, false)
	if err == nil {
		t.Fatal("expected error from Prune")
	}
	assertContains(t, err, "prune:")
}

func TestRemoveWorktreeForce(t *testing.T) {
	var captured []string
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			captured = args
			return "", nil
		},
	}

	if err := RemoveWorktree(context.Background(), r, fakePath, true); err != nil {
		t.Fatalf("RemoveWorktree: %v", err)
	}
	if !slices.Contains(captured, flagForce) {
		t.Errorf(errExpectedInFmt, flagForce, captured)
	}
}

func TestMoveWorktreeInsertsDashDash(t *testing.T) {
	var captured []string
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			captured = args
			return "", nil
		},
	}

	if err := MoveWorktree(r, "/old/path", "/new/path", false); err != nil {
		t.Fatalf("MoveWorktree: %v", err)
	}

	want := []string{cmdWorktree, "move", "--", "/old/path", "/new/path"}
	if !slices.Equal(captured, want) {
		t.Errorf("args = %v, want %v", captured, want)
	}
}

func TestMoveWorktreeForce(t *testing.T) {
	var captured []string
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			captured = args
			return "", nil
		},
	}

	if err := MoveWorktree(r, "/old/path", "/new/path", true); err != nil {
		t.Fatalf("MoveWorktree: %v", err)
	}
	// git worktree move requires --force twice to move locked worktrees
	want := []string{cmdWorktree, "move", flagForce, flagForce, "--", "/old/path", "/new/path"}
	if !slices.Equal(captured, want) {
		t.Errorf("args = %v, want %v", captured, want)
	}
}

func TestListWorktreesError(t *testing.T) {
	r := &mockRunner{
		run: func(_ ...string) (string, error) {
			return "", errors.New(errNotARepo)
		},
	}

	entries, err := ListWorktrees(context.Background(), r)
	if err == nil {
		t.Fatal("expected error from ListWorktrees")
	}
	if entries != nil {
		t.Errorf("expected nil entries, got %v", entries)
	}
}

// defaultBranchRunner returns a mockRunner that resolves symbolic-ref to the
// given symRef (or errors if empty), and accepts rev-parse --verify for the
// specified acceptBranch (or rejects all if empty).
func defaultBranchRunner(symRef, acceptBranch string) *mockRunner {
	return &mockRunner{
		run: func(args ...string) (string, error) {
			if args[0] == "symbolic-ref" {
				if symRef != "" {
					return symRef, nil
				}
				return "", errors.New("no symbolic ref")
			}
			if acceptBranch != "" && args[0] == cmdRevParse && args[2] == refsHeadsPrefix+acceptBranch {
				return "", nil
			}
			return "", errors.New("not found")
		},
	}
}

func TestDefaultBranchSymbolicRef(t *testing.T) {
	r := defaultBranchRunner("refs/remotes/origin/develop", "")
	branch, err := DefaultBranch(context.Background(), r)
	if err != nil {
		t.Fatalf(fatalDefaultFmt, err)
	}
	if branch != "develop" {
		t.Errorf(errBranchWantFmt, branch, "develop")
	}
}

func TestDefaultBranchFallbackMain(t *testing.T) {
	r := defaultBranchRunner("", "main")
	branch, err := DefaultBranch(context.Background(), r)
	if err != nil {
		t.Fatalf(fatalDefaultFmt, err)
	}
	if branch != "main" {
		t.Errorf(errBranchWantFmt, branch, "main")
	}
}

func TestDefaultBranchFallbackMaster(t *testing.T) {
	r := defaultBranchRunner("", "master")
	branch, err := DefaultBranch(context.Background(), r)
	if err != nil {
		t.Fatalf(fatalDefaultFmt, err)
	}
	if branch != "master" {
		t.Errorf(errBranchWantFmt, branch, "master")
	}
}

func TestDefaultBranchNotFound(t *testing.T) {
	r := defaultBranchRunner("", "")
	_, err := DefaultBranch(context.Background(), r)
	if err == nil {
		t.Fatal("expected error when no default branch found")
	}
	assertContains(t, err, "could not detect default branch")
	assertContains(t, err, "To fix:")
	assertContains(t, err, "git branch -M main")
}

func TestHooksDirError(t *testing.T) {
	r := &mockRunner{
		run: func(_ ...string) (string, error) {
			return "", errors.New(errNotARepo)
		},
	}

	_, err := HooksDir(context.Background(), r)
	if err == nil {
		t.Fatal("expected error from HooksDir")
	}
	assertContains(t, err, errNotAGitRepo)
}

func TestHooksDirRelativePath(t *testing.T) {
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			if args[0] == cmdConfig {
				return "", errors.New("not configured")
			}
			if len(args) >= 2 && args[1] == flagGitCommonDir {
				return fakeGitDir, nil
			}
			if len(args) >= 2 && args[1] == flagShowToplevel {
				return fakeRepoDir, nil
			}
			return "", errors.New("unexpected command")
		},
	}

	dir, err := HooksDir(context.Background(), r)
	if err != nil {
		t.Fatalf("HooksDir: %v", err)
	}

	want := filepath.Join(fakeRepoDir, fakeGitDir, "hooks")
	if dir != want {
		t.Errorf("HooksDir = %q, want %q", dir, want)
	}
}

func TestHooksDirAbsolutePath(t *testing.T) {
	absGitDir := filepath.Join(fakeRepoDir, fakeGitDir)
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			if args[0] == cmdConfig {
				return "", errors.New("not configured")
			}
			if len(args) >= 2 && args[1] == flagGitCommonDir {
				return absGitDir, nil
			}
			return "", errors.New("unexpected command")
		},
	}

	dir, err := HooksDir(context.Background(), r)
	if err != nil {
		t.Fatalf("HooksDir: %v", err)
	}

	want := filepath.Join(absGitDir, "hooks")
	if dir != want {
		t.Errorf("HooksDir = %q, want %q", dir, want)
	}
}

func TestRepoRootError(t *testing.T) {
	r := &mockRunner{
		run: func(_ ...string) (string, error) {
			return "", errors.New(errNotARepo)
		},
	}

	_, err := RepoRoot(context.Background(), r)
	if err == nil {
		t.Fatal("expected error from RepoRoot")
	}
	assertContains(t, err, errNotAGitRepo)
	assertContains(t, err, "To fix:")
	assertContains(t, err, "git init")
}

func TestHooksDirRelativePathRepoRootError(t *testing.T) {
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			if args[0] == cmdConfig {
				return "", errors.New("not configured")
			}
			if len(args) >= 2 && args[1] == flagGitCommonDir {
				return fakeGitDir, nil // relative path
			}
			// RepoRoot call fails
			return "", errors.New(errNotARepo)
		},
	}

	_, err := HooksDir(context.Background(), r)
	if err == nil {
		t.Fatal("expected error from HooksDir when RepoRoot fails")
	}
	assertContains(t, err, errNotAGitRepo)
}

func TestHooksDirCoreHooksPathAbsolute(t *testing.T) {
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			if args[0] == cmdConfig {
				return "/custom/hooks", nil
			}
			return "", errors.New("unexpected command")
		},
	}

	dir, err := HooksDir(context.Background(), r)
	if err != nil {
		t.Fatalf("HooksDir: %v", err)
	}

	if dir != "/custom/hooks" {
		t.Errorf("HooksDir = %q, want %q", dir, "/custom/hooks")
	}
}

func TestHooksDirCoreHooksPathRelative(t *testing.T) {
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			if args[0] == cmdConfig {
				return ".githooks", nil
			}
			if len(args) >= 2 && args[1] == flagGitCommonDir {
				return fakeGitDir, nil
			}
			if len(args) >= 2 && args[1] == flagShowToplevel {
				return fakeRepoDir, nil
			}
			return "", errors.New("unexpected command")
		},
	}

	dir, err := HooksDir(context.Background(), r)
	if err != nil {
		t.Fatalf("HooksDir: %v", err)
	}

	want := filepath.Join(fakeRepoDir, ".githooks")
	if dir != want {
		t.Errorf("HooksDir = %q, want %q", dir, want)
	}
}

func TestHooksDirCoreHooksPathRelativeRepoRootError(t *testing.T) {
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			if args[0] == cmdConfig {
				return ".githooks", nil
			}
			// MainRepoRoot → resolveCommonDir fails
			return "", errors.New(errNotARepo)
		},
	}

	_, err := HooksDir(context.Background(), r)
	if err == nil {
		t.Fatal("expected error from HooksDir when MainRepoRoot fails")
	}
}

func TestListWorktreesBareEntry(t *testing.T) {
	output := "worktree /repo\nHEAD abc123\nbranch refs/heads/main\n\nworktree /repo/.git/worktrees/bare\nHEAD def456\nbare\n"
	r := &mockRunner{
		run: func(_ ...string) (string, error) {
			return output, nil
		},
	}

	entries, err := ListWorktrees(context.Background(), r)
	if err != nil {
		t.Fatalf("ListWorktrees: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
	if !entries[1].Bare {
		t.Error("expected second entry to be bare")
	}
	if entries[1].Branch != "" {
		t.Errorf("expected empty branch for bare entry, got %q", entries[1].Branch)
	}
}

func TestIsSquashMerged(t *testing.T) {
	defer func(orig func(context.Context, string) (map[string]bool, error)) {
		ComputePatchIDs = orig
	}(ComputePatchIDs)
	ComputePatchIDs = func(_ context.Context, _ string) (map[string]bool, error) {
		return map[string]bool{fakeSHA: true}, nil
	}

	r := &mockRunner{
		run: func(args ...string) (string, error) {
			switch args[0] {
			case CmdMergeBase:
				return fakeSHA, nil
			case cmdRevParse:
				return fakeTip, nil
			case CmdDiff:
				return "fake diff", nil
			case CmdLog:
				return fakeLog, nil
			}
			return "", errors.New("unexpected call: " + args[0])
		},
	}

	merged, err := IsSquashMerged(context.Background(), r, branchMain, branchFeature)
	if err != nil {
		t.Fatalf("IsSquashMerged: %v", err)
	}
	if !merged {
		t.Error("expected squash-merged branch to be detected")
	}
}

func TestIsSquashMergedNotMerged(t *testing.T) {
	defer func(orig func(context.Context, string) (map[string]bool, error)) {
		ComputePatchIDs = orig
	}(ComputePatchIDs)
	ComputePatchIDs = func(_ context.Context, diff string) (map[string]bool, error) {
		if diff == fakeLog {
			return map[string]bool{"merge-ref-pid": true}, nil
		}
		return map[string]bool{"branch-pid": true}, nil
	}

	r := &mockRunner{
		run: func(args ...string) (string, error) {
			switch args[0] {
			case CmdMergeBase:
				return fakeSHA, nil
			case cmdRevParse:
				return fakeTip, nil
			case CmdDiff:
				return "fake diff", nil
			case CmdLog:
				return fakeLog, nil
			}
			return "", errors.New("unexpected call: " + args[0])
		},
	}

	merged, err := IsSquashMerged(context.Background(), r, branchMain, branchFeature)
	if err != nil {
		t.Fatalf("IsSquashMerged: %v", err)
	}
	if merged {
		t.Error("expected non-squash-merged branch to not be detected")
	}
}

func TestIsSquashMergedMergeBaseError(t *testing.T) {
	r := &mockRunner{
		run: func(_ ...string) (string, error) {
			return "", errors.New(errNotARepo)
		},
	}

	_, err := IsSquashMerged(context.Background(), r, branchMain, branchFeature)
	if err == nil {
		t.Fatal("expected error from MergeBase failure")
	}
}

func TestIsSquashMergedRevParseError(t *testing.T) {
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			if args[0] == CmdMergeBase {
				return fakeSHA, nil
			}
			return "", errors.New("rev-parse failed")
		},
	}

	_, err := IsSquashMerged(context.Background(), r, branchMain, branchFeature)
	if err == nil {
		t.Fatal("expected error from rev-parse failure")
	}
}

func TestIsSquashMergedDiffError(t *testing.T) {
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			switch args[0] {
			case CmdMergeBase:
				return fakeSHA, nil
			case cmdRevParse:
				return fakeTip, nil
			}
			return "", errors.New("diff failed")
		},
	}

	_, err := IsSquashMerged(context.Background(), r, branchMain, branchFeature)
	if err == nil {
		t.Fatal("expected error from diff failure")
	}
}

func TestIsSquashMergedEmptyBranchPatchID(t *testing.T) {
	defer func(orig func(context.Context, string) (map[string]bool, error)) {
		ComputePatchIDs = orig
	}(ComputePatchIDs)
	ComputePatchIDs = func(_ context.Context, _ string) (map[string]bool, error) {
		return map[string]bool{}, nil
	}

	r := &mockRunner{
		run: func(args ...string) (string, error) {
			switch args[0] {
			case CmdMergeBase:
				return fakeSHA, nil
			case cmdRevParse:
				return fakeTip, nil
			case CmdDiff:
				return fakeDiff, nil
			}
			return "", errors.New("unexpected call: " + args[0])
		},
	}

	merged, err := IsSquashMerged(context.Background(), r, branchMain, branchFeature)
	if err != nil {
		t.Fatalf("IsSquashMerged: %v", err)
	}
	if merged {
		t.Error("expected empty branch patch-id to not indicate squash-merge")
	}
}

func TestIsSquashMergedLogError(t *testing.T) {
	defer func(orig func(context.Context, string) (map[string]bool, error)) {
		ComputePatchIDs = orig
	}(ComputePatchIDs)
	ComputePatchIDs = func(_ context.Context, _ string) (map[string]bool, error) {
		return map[string]bool{fakeSHA: true}, nil
	}

	r := &mockRunner{
		run: func(args ...string) (string, error) {
			switch args[0] {
			case CmdMergeBase:
				return fakeSHA, nil
			case cmdRevParse:
				return fakeTip, nil
			case CmdDiff:
				return fakeDiff, nil
			}
			return "", errors.New("log failed")
		},
	}

	_, err := IsSquashMerged(context.Background(), r, branchMain, branchFeature)
	if err == nil {
		t.Fatal("expected error from log failure")
	}
}

func TestIsSquashMergedPatchIDError(t *testing.T) {
	defer func(orig func(context.Context, string) (map[string]bool, error)) {
		ComputePatchIDs = orig
	}(ComputePatchIDs)
	ComputePatchIDs = func(_ context.Context, _ string) (map[string]bool, error) {
		return nil, errors.New("patch-id failed")
	}

	r := &mockRunner{
		run: func(args ...string) (string, error) {
			switch args[0] {
			case CmdMergeBase:
				return fakeSHA, nil
			case cmdRevParse:
				return fakeTip, nil
			case CmdDiff:
				return fakeDiff, nil
			}
			return "", errors.New("unexpected call: " + args[0])
		},
	}

	_, err := IsSquashMerged(context.Background(), r, branchMain, branchFeature)
	if err == nil {
		t.Fatal("expected error from patch-id failure")
	}
}

func TestIsSquashMergedPatchIDErrorOnMergeRef(t *testing.T) {
	defer func(orig func(context.Context, string) (map[string]bool, error)) {
		ComputePatchIDs = orig
	}(ComputePatchIDs)
	ComputePatchIDs = func(_ context.Context, diff string) (map[string]bool, error) {
		if diff == fakeLog {
			return nil, errors.New("patch-id failed on mergeRef")
		}
		return map[string]bool{fakeSHA: true}, nil
	}

	r := &mockRunner{
		run: func(args ...string) (string, error) {
			switch args[0] {
			case CmdMergeBase:
				return fakeSHA, nil
			case cmdRevParse:
				return fakeTip, nil
			case CmdDiff:
				return fakeDiff, nil
			case CmdLog:
				return fakeLog, nil
			}
			return "", errors.New("unexpected call: " + args[0])
		},
	}

	_, err := IsSquashMerged(context.Background(), r, branchMain, branchFeature)
	if err == nil {
		t.Fatal("expected error from patch-id failure on mergeRef diffs")
	}
}

func TestIsSquashMergedMergeBaseEqualsTip(t *testing.T) {
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			switch args[0] {
			case CmdMergeBase:
				return fakeSHA, nil
			case cmdRevParse:
				return fakeSHA, nil // same SHA = empty branch
			}
			return "", errors.New("unexpected call: " + args[0])
		},
	}

	merged, err := IsSquashMerged(context.Background(), r, branchMain, branchFeature)
	if err != nil {
		t.Fatalf("IsSquashMerged: %v", err)
	}
	if merged {
		t.Error("expected empty branch (merge-base == tip) to not indicate squash-merge")
	}
}

func TestMainRepoRootError(t *testing.T) {
	r := &mockRunner{
		run: func(_ ...string) (string, error) {
			return "", errors.New(errNotARepo)
		},
	}

	_, err := MainRepoRoot(context.Background(), r)
	if err == nil {
		t.Fatal("expected error from MainRepoRoot")
	}
	assertContains(t, err, errNotAGitRepo)
}

func TestMainRepoRootRelativePath(t *testing.T) {
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			if len(args) >= 2 && args[1] == flagGitCommonDir {
				return fakeGitDir, nil
			}
			if len(args) >= 2 && args[1] == flagShowToplevel {
				return fakeRepoDir, nil
			}
			return "", errors.New("unexpected command")
		},
	}

	root, err := MainRepoRoot(context.Background(), r)
	if err != nil {
		t.Fatalf("MainRepoRoot: %v", err)
	}

	if root != fakeRepoDir {
		t.Errorf("MainRepoRoot = %q, want %q", root, fakeRepoDir)
	}
}

func TestMainRepoRootAbsolutePath(t *testing.T) {
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			if len(args) >= 2 && args[1] == flagGitCommonDir {
				return filepath.Join(fakeRepoDir, fakeGitDir), nil
			}
			return "", errors.New("unexpected command")
		},
	}

	root, err := MainRepoRoot(context.Background(), r)
	if err != nil {
		t.Fatalf("MainRepoRoot: %v", err)
	}

	if root != fakeRepoDir {
		t.Errorf("MainRepoRoot = %q, want %q", root, fakeRepoDir)
	}
}

func TestMainRepoRootRelativePathRepoRootError(t *testing.T) {
	callCount := 0
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			callCount++
			if len(args) >= 2 && args[1] == flagGitCommonDir {
				return fakeGitDir, nil // relative path
			}
			// RepoRoot call fails
			return "", errors.New(errNotARepo)
		},
	}

	_, err := MainRepoRoot(context.Background(), r)
	if err == nil {
		t.Fatal("expected error from MainRepoRoot when RepoRoot fails")
	}
	assertContains(t, err, errNotAGitRepo)
}

func TestLastCommitInfoEmptyOutput(t *testing.T) {
	r := &mockRunner{
		run: func(_ ...string) (string, error) {
			return "", nil
		},
	}
	_, _, err := LastCommitInfo(context.Background(), r, branchFeature)
	if err == nil {
		t.Fatal("expected error for empty output")
	}
	assertContains(t, err, "no commits on branch")
}

func TestLastCommitInfoMalformed(t *testing.T) {
	r := &mockRunner{
		run: func(_ ...string) (string, error) {
			return "no-tab-separator", nil
		},
	}
	_, _, err := LastCommitInfo(context.Background(), r, branchFeature)
	if err == nil {
		t.Fatal("expected error for malformed output")
	}
	assertContains(t, err, "malformed commit info")
}

func TestLastCommitInfoBadTimestamp(t *testing.T) {
	r := &mockRunner{
		run: func(_ ...string) (string, error) {
			return "not-a-number\tcommit subject", nil
		},
	}
	_, _, err := LastCommitInfo(context.Background(), r, branchFeature)
	if err == nil {
		t.Fatal("expected error for non-numeric timestamp")
	}
	assertContains(t, err, "parse commit timestamp")
}

func TestLastCommitInfoRunError(t *testing.T) {
	r := &mockRunner{
		run: func(_ ...string) (string, error) {
			return "", errors.New(errNotARepo)
		},
	}
	_, _, err := LastCommitInfo(context.Background(), r, branchFeature)
	if err == nil {
		t.Fatal("expected error from runner failure")
	}
	assertContains(t, err, "last commit info")
	assertContains(t, err, "To fix:")
}

func TestLocalBranchesError(t *testing.T) {
	r := &mockRunner{
		run: func(_ ...string) (string, error) {
			return "", errors.New(errNotARepo)
		},
	}
	_, err := LocalBranches(context.Background(), r)
	if err == nil {
		t.Fatal("expected error from LocalBranches")
	}
}

func TestRepoNameError(t *testing.T) {
	r := &mockRunner{
		run: func(_ ...string) (string, error) {
			return "", errors.New(errNotARepo)
		},
	}

	_, err := RepoName(context.Background(), r)
	if err == nil {
		t.Fatal("expected error from RepoName")
	}
	assertContains(t, err, errNotAGitRepo)
}

func TestCheckoutInsertsDashDash(t *testing.T) {
	const branch = "my-branch"
	var capturedDir string
	var capturedArgs []string
	r := &mockRunner{
		runInDir: func(dir string, args ...string) (string, error) {
			capturedDir = dir
			capturedArgs = args
			return "", nil
		},
	}

	if err := Checkout(context.Background(), r, fakeDir, branch); err != nil {
		t.Fatalf("Checkout: %v", err)
	}

	want := []string{"switch", "--", branch}
	if !slices.Equal(capturedArgs, want) {
		t.Errorf("args = %v, want %v", capturedArgs, want)
	}
	if capturedDir != fakeDir {
		t.Errorf("dir = %q, want %q", capturedDir, fakeDir)
	}
}

func TestAddWorktreeFromBranchInsertsDashDash(t *testing.T) {
	const branch = "feature/some-task"
	var capturedArgs []string
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			capturedArgs = args
			return "", nil
		},
	}

	if err := AddWorktreeFromBranch(context.Background(), r, fakePath, branch); err != nil {
		t.Fatalf("AddWorktreeFromBranch: %v", err)
	}

	want := []string{cmdWorktree, "add", "--", fakePath, branch}
	if !slices.Equal(capturedArgs, want) {
		t.Errorf("args = %v, want %v", capturedArgs, want)
	}
}

func TestAddWorktreeInsertsDashDash(t *testing.T) {
	const (
		branch = "feature/my-task"
		source = "main"
	)
	var capturedArgs []string
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			capturedArgs = args
			return "", nil
		},
	}

	if err := AddWorktree(context.Background(), r, fakePath, branch, source); err != nil {
		t.Fatalf("AddWorktree: %v", err)
	}

	want := []string{cmdWorktree, "add", "-b", branch, "--", fakePath, source}
	if !slices.Equal(capturedArgs, want) {
		t.Errorf("args = %v, want %v", capturedArgs, want)
	}
}

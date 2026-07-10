package git

import (
	"context"
	"errors"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
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
	fakeDiffText     = "fake diff"
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
	var captured []string
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			captured = args
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

	wantArgs := []string{"branch", "--merged=" + branchMain}
	if !slices.Equal(captured, wantArgs) {
		t.Errorf("args = %v, want %v", captured, wantArgs)
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

	want := []string{"branch", flagForceD, "--", branchOld}
	if !slices.Equal(captured, want) {
		t.Errorf("args = %v, want %v", captured, want)
	}
}

func TestDeleteBranchAlreadyGoneIsIdempotent(t *testing.T) {
	var calls [][]string
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			calls = append(calls, args)
			// Both calls fail — simulates a branch that's already gone.
			return "", errors.New("fatal: needed a single revision")
		},
	}
	if err := DeleteBranch(context.Background(), r, "gone", false); err != nil {
		t.Fatalf("expected nil for already-gone branch, got: %v", err)
	}
	if len(calls) != 2 {
		t.Fatalf("expected delete attempt + fallback existence check, got %d calls: %v", len(calls), calls)
	}
	if calls[0][0] != "branch" {
		t.Errorf("expected delete attempted first (act-then-check, not check-then-act), got %v", calls[0])
	}
	if calls[1][0] != cmdRevParse {
		t.Errorf("expected existence check as the fallback on delete failure, got %v", calls[1])
	}
}

func TestDeleteBranchOtherErrorPropagates(t *testing.T) {
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			if args[0] == cmdRevParse {
				return "", nil // branch exists — delete failure is a real error, not "already gone"
			}
			return "", errors.New("error: Cannot delete branch 'main' checked out at '/repo'")
		},
	}
	if err := DeleteBranch(context.Background(), r, "main", false); err == nil {
		t.Fatal("expected non-nil error for checked-out branch")
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

	want := []string{"branch", "-m", "--", branchOld, branchNew}
	if !slices.Equal(captured, want) {
		t.Errorf("args = %v, want %v", captured, want)
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
	want := []string{cmdWorktree, "remove", flagForce, "--", fakePath}
	if !slices.Equal(captured, want) {
		t.Errorf("args = %v, want %v", captured, want)
	}
}

func TestRemoveWorktreeNoForceInsertsDashDash(t *testing.T) {
	var captured []string
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			captured = args
			return "", nil
		},
	}

	if err := RemoveWorktree(context.Background(), r, fakePath, false); err != nil {
		t.Fatalf("RemoveWorktree: %v", err)
	}
	want := []string{cmdWorktree, "remove", "--", fakePath}
	if !slices.Equal(captured, want) {
		t.Errorf("args = %v, want %v", captured, want)
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

	if err := MoveWorktree(context.Background(), r, "/old/path", "/new/path", false); err != nil {
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

	if err := MoveWorktree(context.Background(), r, "/old/path", "/new/path", true); err != nil {
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
				return fakeDiffText, nil
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

func TestIsSquashMergedWithMainlinePatchIDsMatch(t *testing.T) {
	defer func(orig func(context.Context, string) (map[string]bool, error)) {
		ComputePatchIDs = orig
	}(ComputePatchIDs)
	ComputePatchIDs = func(_ context.Context, _ string) (map[string]bool, error) {
		return map[string]bool{fakeSHA: true}, nil
	}

	r := &mockRunner{
		run: func(args ...string) (string, error) {
			switch args[0] {
			case cmdRevParse:
				return fakeTip, nil
			case CmdDiff:
				return fakeDiffText, nil
			}
			return "", errors.New("unexpected call: " + args[0]) // no MergeBase/CmdLog: mainline is precomputed
		},
	}

	mainlinePIDs := map[string]bool{fakeSHA: true}
	merged, err := IsSquashMergedWithMainlinePatchIDs(context.Background(), r, fakeSHA, branchFeature, mainlinePIDs)
	if err != nil {
		t.Fatalf("IsSquashMergedWithMainlinePatchIDs: %v", err)
	}
	if !merged {
		t.Error("expected squash-merged branch to be detected against the precomputed mainline set")
	}
}

func TestIsSquashMergedWithMainlinePatchIDsNoMatch(t *testing.T) {
	defer func(orig func(context.Context, string) (map[string]bool, error)) {
		ComputePatchIDs = orig
	}(ComputePatchIDs)
	ComputePatchIDs = func(_ context.Context, _ string) (map[string]bool, error) {
		return map[string]bool{"branch-only-pid": true}, nil
	}

	r := &mockRunner{
		run: func(args ...string) (string, error) {
			switch args[0] {
			case cmdRevParse:
				return fakeTip, nil
			case CmdDiff:
				return fakeDiffText, nil
			}
			return "", errors.New("unexpected call: " + args[0])
		},
	}

	mainlinePIDs := map[string]bool{"mainline-only-pid": true}
	merged, err := IsSquashMergedWithMainlinePatchIDs(context.Background(), r, fakeSHA, branchFeature, mainlinePIDs)
	if err != nil {
		t.Fatalf("IsSquashMergedWithMainlinePatchIDs: %v", err)
	}
	if merged {
		t.Error("expected no match against a disjoint precomputed mainline set")
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
				return fakeDiffText, nil
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

func TestCommonDirRelativePath(t *testing.T) {
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

	want := filepath.Join(fakeRepoDir, fakeGitDir)
	got, err := CommonDir(context.Background(), r)
	if err != nil {
		t.Fatalf("CommonDir: %v", err)
	}
	if got != want {
		t.Errorf("CommonDir = %q, want %q", got, want)
	}
}

func TestCommonDirAbsolutePath(t *testing.T) {
	want := filepath.Join(fakeRepoDir, fakeGitDir)
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			if len(args) >= 2 && args[1] == flagGitCommonDir {
				return want, nil
			}
			return "", errors.New("unexpected command")
		},
	}

	got, err := CommonDir(context.Background(), r)
	if err != nil {
		t.Fatalf("CommonDir: %v", err)
	}
	if got != want {
		t.Errorf("CommonDir = %q, want %q", got, want)
	}
}

func TestCommonDirError(t *testing.T) {
	r := &mockRunner{
		run: func(_ ...string) (string, error) {
			return "", errors.New(errNotARepo)
		},
	}

	_, err := CommonDir(context.Background(), r)
	if err == nil {
		t.Fatal("expected error from CommonDir")
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

func TestFirstParentChainSHAsParsesLines(t *testing.T) {
	var captured []string
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			captured = args
			if args[0] == cmdRevList {
				return "sha1\n" + fakeTip + "\n\nsha2\n", nil
			}
			return "", errors.New("unexpected call: " + args[0])
		},
	}

	shas, err := FirstParentChainSHAs(context.Background(), r, branchMain)
	if err != nil {
		t.Fatalf("FirstParentChainSHAs: %v", err)
	}
	want := []string{"sha1", fakeTip, "sha2"}
	if len(shas) != len(want) {
		t.Fatalf("shas = %v, want %v", shas, want)
	}
	for _, sha := range want {
		if !shas[sha] {
			t.Errorf("missing %q in %v", sha, shas)
		}
	}

	wantArgs := []string{cmdRevList, flagFirstParent, flagEndOfOptions, branchMain}
	if !slices.Equal(captured, wantArgs) {
		t.Errorf("args = %v, want %v", captured, wantArgs)
	}
}

func TestMainlinePatchIDsSinceArgs(t *testing.T) {
	var captured []string
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			captured = args
			return "", nil
		},
	}

	if _, err := MainlinePatchIDsSince(context.Background(), r, fakeSHA, branchMain); err != nil {
		t.Fatalf("MainlinePatchIDsSince: %v", err)
	}

	want := []string{CmdLog, "-p", "--no-merges", flagEndOfOptions, fakeSHA + ".." + branchMain}
	if !slices.Equal(captured, want) {
		t.Errorf("args = %v, want %v", captured, want)
	}
}

func TestLastCommitInfoArgs(t *testing.T) {
	var captured []string
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			captured = args
			return "123\tsubject", nil
		},
	}

	if _, _, err := LastCommitInfo(context.Background(), r, branchFeature); err != nil {
		t.Fatalf("LastCommitInfo: %v", err)
	}

	want := []string{CmdLog, "-1", "--format=%ct\t%s", flagEndOfOptions, branchFeature}
	if !slices.Equal(captured, want) {
		t.Errorf("args = %v, want %v", captured, want)
	}
}

func TestCommitCountSinceArgs(t *testing.T) {
	var captured []string
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			captured = args
			return "3", nil
		},
	}

	if _, err := CommitCountSince(context.Background(), r, branchFeature, 24*time.Hour); err != nil {
		t.Fatalf("CommitCountSince: %v", err)
	}

	if len(captured) != 5 || captured[0] != "rev-list" || captured[1] != "--count" || captured[3] != flagEndOfOptions || captured[4] != branchFeature {
		t.Errorf("args = %v, want [rev-list --count --since=<ts> %s %s]", captured, flagEndOfOptions, branchFeature)
	}
}

func TestBranchOwnPatchIDsArgTerminators(t *testing.T) {
	defer func(orig func(context.Context, string) (map[string]bool, error)) {
		ComputePatchIDs = orig
	}(ComputePatchIDs)
	ComputePatchIDs = func(_ context.Context, _ string) (map[string]bool, error) {
		return map[string]bool{fakeSHA: true}, nil
	}

	var revParseArgs, diffArgs []string
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			switch args[0] {
			case cmdRevParse:
				revParseArgs = args
				return fakeTip, nil
			case CmdDiff:
				diffArgs = args
				return fakeDiffText, nil
			}
			return "", errors.New("unexpected call: " + args[0])
		},
	}

	if _, _, err := branchOwnPatchIDs(context.Background(), r, fakeSHA, branchFeature); err != nil {
		t.Fatalf("branchOwnPatchIDs: %v", err)
	}

	wantRevParse := []string{cmdRevParse, flagVerify, flagEndOfOptions, branchFeature}
	if !slices.Equal(revParseArgs, wantRevParse) {
		t.Errorf("rev-parse args = %v, want %v", revParseArgs, wantRevParse)
	}
	wantDiff := []string{CmdDiff, flagEndOfOptions, fakeSHA, branchFeature}
	if !slices.Equal(diffArgs, wantDiff) {
		t.Errorf("diff args = %v, want %v", diffArgs, wantDiff)
	}
}

func TestFirstParentChainSHAsError(t *testing.T) {
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			return "", errors.New("rev-list failed")
		},
	}

	if _, err := FirstParentChainSHAs(context.Background(), r, branchMain); err == nil {
		t.Fatal("expected error from rev-list failure")
	}
}

func TestIsSHAOnChain(t *testing.T) {
	chain := map[string]bool{fakeTip: true}
	if !IsSHAOnChain(fakeTip, chain) {
		t.Error("expected true for a SHA present in the chain")
	}
	if IsSHAOnChain("missing", chain) {
		t.Error("expected false for a SHA absent from the chain")
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

func TestMergeInsertsDashDash(t *testing.T) {
	const branch = "feat/x"
	var capturedArgs []string
	r := &mockRunner{
		runInDir: func(_ string, args ...string) (string, error) {
			capturedArgs = args
			return "", nil
		},
	}

	if err := Merge(context.Background(), r, fakeDir, branch, false); err != nil {
		t.Fatalf("Merge: %v", err)
	}

	want := []string{"merge", "--", branch}
	if !slices.Equal(capturedArgs, want) {
		t.Errorf("args = %v, want %v", capturedArgs, want)
	}
}

func TestMergeNoFFInsertsDashDash(t *testing.T) {
	const branch = "feat/x"
	var capturedArgs []string
	r := &mockRunner{
		runInDir: func(_ string, args ...string) (string, error) {
			capturedArgs = args
			return "", nil
		},
	}

	if err := Merge(context.Background(), r, fakeDir, branch, true); err != nil {
		t.Fatalf("Merge: %v", err)
	}

	want := []string{"merge", "--no-ff", "--", branch}
	if !slices.Equal(capturedArgs, want) {
		t.Errorf("args = %v, want %v", capturedArgs, want)
	}
}

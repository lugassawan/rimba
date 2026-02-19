package git

import (
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
	fakeTree         = "tree123"
	fakeTempCommit   = "temp456"
)

// mockRunner implements Runner with configurable closures for testing.
type mockRunner struct {
	run      func(args ...string) (string, error)
	runInDir func(dir string, args ...string) (string, error)
}

func (m *mockRunner) Run(args ...string) (string, error) {
	return m.run(args...)
}

func (m *mockRunner) RunInDir(dir string, args ...string) (string, error) {
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
			got := parseCount(tt.input, &v)
			if got != tt.want {
				t.Errorf("parseCount(%q) = %d, want %d", tt.input, got, tt.want)
			}
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
			ahead, behind, err := AheadBehind(r, fakeDir)
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

	branches, err := MergedBranches(r, branchMain)
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

	_, err := MergedBranches(r, branchMain)
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

	branches, err := MergedBranches(r, branchMain)
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

	if err := DeleteBranch(r, branchOld, true); err != nil {
		t.Fatalf("DeleteBranch: %v", err)
	}

	if len(captured) != 3 || captured[1] != flagForceD {
		t.Errorf("expected flag %s, got args %v", flagForceD, captured)
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

	if err := RenameBranch(r, branchOld, branchNew); err != nil {
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

	_, err := IsDirty(r, fakeDir)
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

	if _, err := Prune(r, false); err != nil {
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

	out, err := Prune(r, true)
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

	_, err := Prune(r, false)
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

	if err := RemoveWorktree(r, fakePath, true); err != nil {
		t.Fatalf("RemoveWorktree: %v", err)
	}
	if !slices.Contains(captured, flagForce) {
		t.Errorf(errExpectedInFmt, flagForce, captured)
	}
}

func TestMoveWorktreeNoForce(t *testing.T) {
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
	if slices.Contains(captured, flagForce) {
		t.Error("--force should not be present when force=false")
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
	forceCount := 0
	for _, a := range captured {
		if a == flagForce {
			forceCount++
		}
	}
	if forceCount != 2 {
		t.Errorf("expected 2 %s flags, got %d in args %v", flagForce, forceCount, captured)
	}
}

func TestListWorktreesError(t *testing.T) {
	r := &mockRunner{
		run: func(_ ...string) (string, error) {
			return "", errors.New(errNotARepo)
		},
	}

	entries, err := ListWorktrees(r)
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
	branch, err := DefaultBranch(r)
	if err != nil {
		t.Fatalf(fatalDefaultFmt, err)
	}
	if branch != "develop" {
		t.Errorf(errBranchWantFmt, branch, "develop")
	}
}

func TestDefaultBranchFallbackMain(t *testing.T) {
	r := defaultBranchRunner("", "main")
	branch, err := DefaultBranch(r)
	if err != nil {
		t.Fatalf(fatalDefaultFmt, err)
	}
	if branch != "main" {
		t.Errorf(errBranchWantFmt, branch, "main")
	}
}

func TestDefaultBranchFallbackMaster(t *testing.T) {
	r := defaultBranchRunner("", "master")
	branch, err := DefaultBranch(r)
	if err != nil {
		t.Fatalf(fatalDefaultFmt, err)
	}
	if branch != "master" {
		t.Errorf(errBranchWantFmt, branch, "master")
	}
}

func TestDefaultBranchNotFound(t *testing.T) {
	r := defaultBranchRunner("", "")
	_, err := DefaultBranch(r)
	if err == nil {
		t.Fatal("expected error when no default branch found")
	}
	assertContains(t, err, "could not detect default branch")
}

func TestHooksDirError(t *testing.T) {
	r := &mockRunner{
		run: func(_ ...string) (string, error) {
			return "", errors.New(errNotARepo)
		},
	}

	_, err := HooksDir(r)
	if err == nil {
		t.Fatal("expected error from HooksDir")
	}
	assertContains(t, err, errNotAGitRepo)
}

func TestHooksDirRelativePath(t *testing.T) {
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			if len(args) >= 2 && args[1] == flagGitCommonDir {
				return fakeGitDir, nil
			}
			if len(args) >= 2 && args[1] == "--show-toplevel" {
				return fakeRepoDir, nil
			}
			return "", errors.New("unexpected command")
		},
	}

	dir, err := HooksDir(r)
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
			if len(args) >= 2 && args[1] == flagGitCommonDir {
				return absGitDir, nil
			}
			return "", errors.New("unexpected command")
		},
	}

	dir, err := HooksDir(r)
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

	_, err := RepoRoot(r)
	if err == nil {
		t.Fatal("expected error from RepoRoot")
	}
	assertContains(t, err, errNotAGitRepo)
}

func TestHooksDirRelativePathRepoRootError(t *testing.T) {
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

	_, err := HooksDir(r)
	if err == nil {
		t.Fatal("expected error from HooksDir when RepoRoot fails")
	}
	assertContains(t, err, errNotAGitRepo)
}

func TestListWorktreesBareEntry(t *testing.T) {
	output := "worktree /repo\nHEAD abc123\nbranch refs/heads/main\n\nworktree /repo/.git/worktrees/bare\nHEAD def456\nbare\n"
	r := &mockRunner{
		run: func(_ ...string) (string, error) {
			return output, nil
		},
	}

	entries, err := ListWorktrees(r)
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
	step := 0
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			step++
			switch step {
			case 1: // merge-base
				return fakeSHA, nil
			case 2: // rev-parse branch^{tree}
				return fakeTree, nil
			case 3: // commit-tree
				return fakeTempCommit, nil
			case 4: // cherry
				return "- abc789", nil
			}
			return "", errors.New("unexpected call")
		},
	}

	merged, err := IsSquashMerged(r, branchMain, branchFeature)
	if err != nil {
		t.Fatalf("IsSquashMerged: %v", err)
	}
	if !merged {
		t.Error("expected squash-merged branch to be detected")
	}
}

func TestIsSquashMergedNotMerged(t *testing.T) {
	step := 0
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			step++
			switch step {
			case 1:
				return fakeSHA, nil
			case 2:
				return fakeTree, nil
			case 3:
				return fakeTempCommit, nil
			case 4:
				return "+ abc789", nil
			}
			return "", errors.New("unexpected call")
		},
	}

	merged, err := IsSquashMerged(r, branchMain, branchFeature)
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

	_, err := IsSquashMerged(r, branchMain, branchFeature)
	if err == nil {
		t.Fatal("expected error from MergeBase failure")
	}
}

func TestIsSquashMergedRevParseError(t *testing.T) {
	step := 0
	r := &mockRunner{
		run: func(_ ...string) (string, error) {
			step++
			if step == 1 {
				return fakeSHA, nil
			}
			return "", errors.New("rev-parse failed")
		},
	}

	_, err := IsSquashMerged(r, branchMain, branchFeature)
	if err == nil {
		t.Fatal("expected error from rev-parse failure")
	}
}

func TestIsSquashMergedCommitTreeError(t *testing.T) {
	step := 0
	r := &mockRunner{
		run: func(_ ...string) (string, error) {
			step++
			switch step {
			case 1:
				return fakeSHA, nil
			case 2:
				return fakeTree, nil
			}
			return "", errors.New("commit-tree failed")
		},
	}

	_, err := IsSquashMerged(r, branchMain, branchFeature)
	if err == nil {
		t.Fatal("expected error from commit-tree failure")
	}
}

func TestIsSquashMergedEmptyCherry(t *testing.T) {
	step := 0
	r := &mockRunner{
		run: func(_ ...string) (string, error) {
			step++
			switch step {
			case 1:
				return fakeSHA, nil
			case 2:
				return fakeTree, nil
			case 3:
				return fakeTempCommit, nil
			case 4:
				return "", nil
			}
			return "", errors.New("unexpected call")
		},
	}

	merged, err := IsSquashMerged(r, branchMain, branchFeature)
	if err != nil {
		t.Fatalf("IsSquashMerged: %v", err)
	}
	if merged {
		t.Error("expected empty cherry output to not indicate squash-merge")
	}
}

func TestIsSquashMergedCherryError(t *testing.T) {
	step := 0
	r := &mockRunner{
		run: func(_ ...string) (string, error) {
			step++
			switch step {
			case 1:
				return fakeSHA, nil
			case 2:
				return fakeTree, nil
			case 3:
				return fakeTempCommit, nil
			}
			return "", errors.New("cherry failed")
		},
	}

	_, err := IsSquashMerged(r, branchMain, branchFeature)
	if err == nil {
		t.Fatal("expected error from cherry failure")
	}
}

func TestMainRepoRootError(t *testing.T) {
	r := &mockRunner{
		run: func(_ ...string) (string, error) {
			return "", errors.New(errNotARepo)
		},
	}

	_, err := MainRepoRoot(r)
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
			if len(args) >= 2 && args[1] == "--show-toplevel" {
				return fakeRepoDir, nil
			}
			return "", errors.New("unexpected command")
		},
	}

	root, err := MainRepoRoot(r)
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

	root, err := MainRepoRoot(r)
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

	_, err := MainRepoRoot(r)
	if err == nil {
		t.Fatal("expected error from MainRepoRoot when RepoRoot fails")
	}
	assertContains(t, err, errNotAGitRepo)
}

func TestRepoNameError(t *testing.T) {
	r := &mockRunner{
		run: func(_ ...string) (string, error) {
			return "", errors.New(errNotARepo)
		},
	}

	_, err := RepoName(r)
	if err == nil {
		t.Fatal("expected error from RepoName")
	}
	assertContains(t, err, errNotAGitRepo)
}
